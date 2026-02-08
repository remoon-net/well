package hookjs

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/PuerkitoBio/goquery"
	"github.com/dop251/goja"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/jsvm"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/store"
	"github.com/shynome/err0"
	"github.com/shynome/err0/try"
	almond "github.com/shynome/goja-almond"
	"remoon.net/well/db"
)

var deps atomic.Pointer[goja.Program]

func updateDeps(app core.App) error {
	cc, err := app.FindAllCollections()
	if err != nil {
		return err
	}
	dd := []string{}
	for _, c := range cc {
		if c.System {
			continue
		}
		dd = append(dd, c.Name)
	}
	p, err := genDeps(dd)
	if err != nil {
		return err
	}
	deps.Store(p)
	return nil
}

func genDeps(collections []string) (*goja.Program, error) {
	mods := "var depend_collections = [];\n"
	for _, c := range collections {
		mods += fmt.Sprintf("define('collections/%s',()=>{ depend_collections.push('%s') });\n", c, c)
	}
	p, err := goja.Compile("collections", mods, false)
	return p, err
}

func InitHookJS(app core.App) error {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := updateDeps(e.App); err != nil {
			return err
		}
		q := dbx.HashExp{"disabled": false}
		records, err := app.FindAllRecords(db.TableHookJS, q)
		if err != nil {
			return err
		}
		logger := e.App.Logger()
		for _, r := range records {
			removeHookJS(e.App, r.Id)
			if err := addHookJS(e.App, r); err != nil {
				logger.Error("add hookjs failed.", "record", r, "error", err)
			}
		}
		return e.Next()
	})
	// handle collections update
	app.OnCollectionAfterCreateSuccess().BindFunc(func(e *core.CollectionEvent) error {
		if err := updateDeps(e.App); err != nil {
			return err
		}
		return e.Next()
	})
	app.OnCollectionAfterUpdateSuccess().BindFunc(func(e *core.CollectionEvent) error {
		if err := updateDeps(e.App); err != nil {
			return err
		}
		return e.Next()
	})
	app.OnCollectionAfterDeleteSuccess().BindFunc(func(e *core.CollectionEvent) error {
		if err := updateDeps(e.App); err != nil {
			return err
		}
		return e.Next()
	})
	// handle hookjs change
	app.OnRecordCreateRequest(db.TableHookJS).BindFunc(func(e *core.RecordRequestEvent) error {
		e.Record.Set("disabled", true) // 先禁用, 后面手动开启
		return e.Next()
	})
	app.OnRecordUpdateRequest(db.TableHookJS).BindFunc(func(e *core.RecordRequestEvent) error {
		removeHookJS(e.App, e.Record.Id)
		if e.Record.GetBool("disabled") {
			return e.Next()
		}
		if err := addHookJS(e.App, e.Record); err != nil {
			return err
		}
		return e.Next()
	})
	app.OnRecordDeleteRequest(db.TableHookJS).BindFunc(func(e *core.RecordRequestEvent) error {
		removeHookJS(e.App, e.Record.Id)
		return e.Next()
	})
	return nil
}

var binds = store.New(map[string]*HookJS{})

func removeHookJS(app core.App, id string) {
	h, ok := binds.GetOk(id)
	if !ok {
		return
	}
	binds.Remove(id)
	h.RemoveListeners(app)
}

func addHookJS(app core.App, record *core.Record) error {
	id := record.Id
	vm, err := newVM()
	if err != nil {
		return err
	}
	hookjs := record.GetString("hookjs")
	h, err := precompile(vm.Runtime, hookjs)
	if err != nil {
		return err
	}
	if len(h.events) == 0 {
		return apis.NewBadRequestError("未监听任何事件, 无需启用", nil)
	}
	h.fill(record)
	binds.Set(id, h)
	h.AddListeners(app)
	return err
}

type HookJS struct {
	*core.Record
	hookjs      *goja.Program
	events      []string
	collections []string
}

func (h *HookJS) fill(r *core.Record) {
	h.Record = r
}

type VM struct {
	*goja.Runtime
	*almond.Module
}

func newVM() (_ *VM, err error) {
	defer err0.Then(&err, nil, nil)

	vm := goja.New()
	mod := try.To1(almond.Enable(vm))
	deps := deps.Load()
	if deps == nil {
		return nil, fmt.Errorf("deps is not ready")
	}
	try.To1(vm.RunProgram(deps))

	vm.SetFieldNameMapper(jsvm.FieldMapper{})

	return &VM{
		Runtime: vm,
		Module:  mod,
	}, nil
}

func (h *HookJS) AddListeners(app core.App) {
	for _, event := range h.events {
		hid := fmt.Sprintf("hookjs-%s-%s", h.Id, event)
		recordRequestHandler := genHandler[*core.RecordRequestEvent](h, hid, event)
		recordEventHandler := genHandler[*core.RecordEvent](h, hid, event)
		switch event {
		case "onRecordValidate":
			app.OnRecordValidate(h.collections...).Bind(recordEventHandler)
		case "onRecordCreate":
			app.OnRecordCreate(h.collections...).Bind(recordEventHandler)
		case "onRecordUpdate":
			app.OnRecordUpdate(h.collections...).Bind(recordEventHandler)
		case "onRecordDelete":
			app.OnRecordDelete(h.collections...).Bind(recordEventHandler)
		case "onRecordCreateRequest":
			app.OnRecordCreateRequest(h.collections...).Bind(recordRequestHandler)
		case "onRecordUpdateRequest":
			app.OnRecordUpdateRequest(h.collections...).Bind(recordRequestHandler)
		case "onRecordDeleteRequest":
			app.OnRecordDeleteRequest(h.collections...).Bind(recordRequestHandler)
		}
	}
}

func (h *HookJS) RemoveListeners(app core.App) {
	for _, event := range h.events {
		hid := fmt.Sprintf("hookjs-%s-%s", h.Id, event)
		switch event {
		case "onRecordValidate":
			app.OnRecordValidate(h.collections...).Unbind(hid)
		case "onRecordCreate":
			app.OnRecordCreate(h.collections...).Unbind(hid)
		case "onRecordUpdate":
			app.OnRecordUpdate(h.collections...).Unbind(hid)
		case "onRecordDelete":
			app.OnRecordDelete(h.collections...).Unbind(hid)
		case "onRecordCreateRequest":
			app.OnRecordCreateRequest(h.collections...).Unbind(hid)
		case "onRecordUpdateRequest":
			app.OnRecordUpdateRequest(h.collections...).Unbind(hid)
		case "onRecordDeleteRequest":
			app.OnRecordDeleteRequest(h.collections...).Unbind(hid)
		}
	}
}

var requireHookJSModule = goja.MustCompile("require_hookjs_module", "requirejs('hookjs')", false)

func genHandler[T hook.Resolver](h *HookJS, hid string, event string) *hook.Handler[T] {
	return &hook.Handler[T]{
		Id:       hid,
		Priority: h.GetInt("order"),
		Func: func(e T) error {
			vm, err := newVM()
			if err != nil {
				return err
			}
			if _, err := vm.RunProgram(h.hookjs); err != nil {
				return err
			}
			m, err := vm.RunProgram(requireHookJSModule)
			if err != nil {
				return err
			}
			mod := m.ToObject(vm.Runtime)
			fn := mod.Get(event)
			call, ok := goja.AssertFunction(fn)
			if !ok {
				return e.Next()
			}
			_, err = call(mod, vm.ToValue(e))
			if err != nil {
				// Unwrap 出来的是 GoError(native), 不用进行包裹
				if err := errors.Unwrap(err); err != nil {
					return err
				}
				msg := fmt.Sprintf("插件 %s(%s) 运行出错了", h.Id, h.GetString("name"))
				return apis.NewInternalServerError(msg, err)
			}
			return nil
		},
	}
}

func precompile(vm *goja.Runtime, hookjs string) (_ *HookJS, err error) {
	defer err0.Then(&err, nil, nil)

	q := try.To1(goquery.NewDocumentFromReader(strings.NewReader(hookjs)))
	script := q.Find("pre.language-javascript>code").Text()

	script = "define.predef = 'hookjs';\n" + script + "\n;define.predef = null;"
	p := try.To1(goja.Compile("hookjs", script, false))

	try.To1(vm.RunProgram(p))
	m := try.To1(vm.RunProgram(requireHookJSModule))

	depsRaw, ok := vm.Get("depend_collections").Export().([]any)
	if !ok {
		return nil, fmt.Errorf("deps should be []any")
	}
	collections := make([]string, len(depsRaw))
	for i, v := range depsRaw {
		v, ok := v.(string)
		if ok {
			collections[i] = v
		}
	}

	events := []string{}
	{
		m := m.ToObject(vm)
		keys := m.Keys()
		for _, k := range keys {
			v := m.Get(k)
			if _, ok := goja.AssertFunction(v); ok {
				events = append(events, k)
			}
		}
	}

	h := &HookJS{
		hookjs:      p,
		collections: collections,
		events:      events,
	}
	return h, nil
}
