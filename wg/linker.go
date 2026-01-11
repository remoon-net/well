package wg

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/hashicorp/yamux"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/store"
	"github.com/shynome/err0"
	"github.com/shynome/err0/try"
	"github.com/shynome/websocket"
	"golang.org/x/crypto/ssh"
	"remoon.net/well/db"
)

var lks = store.New(map[string]*Linker{})

func InitLinkers(app core.App) error {

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		q := dbx.HashExp{"disabled": false}
		linkers, err := se.App.FindAllRecords(db.TableLinkers, q)
		if err != nil {
			return err
		}
		if _, err := se.App.DB().Update(db.TableLinkers, dbx.Params{"status": ""}, dbx.Not(dbx.HashExp{"status": ""})).Execute(); err != nil {
			return err
		}
		for _, r := range linkers {
			lks.GetOrSet(r.Id, linkerInit(se.App, r))
		}
		return se.Next()
	})
	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		for _, lk := range lks.Values() {
			lk.Stop()
		}
		lks.RemoveAll()
		return e.Next()
	})

	preUpdateRequest(app, db.TableLinkers, func(e *core.RecordRequestEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		r := e.Record
		if lk, ok := lks.GetOk(r.Id); ok {
			lks.Remove(r.Id) // 先关闭一次
			lk.Stop()
		}
		if r.GetBool("disabled") {
			return nil // 如果被禁用后则不再启用
		}
		lks.GetOrSet(r.Id, linkerInit(e.App, r))
		return nil
	})
	app.OnRecordDeleteRequest(db.TableLinkers).BindFunc(func(e *core.RecordRequestEvent) error {
		r := e.Record
		if lk, ok := lks.GetOk(r.Id); ok {
			lks.Remove(r.Id)
			lk.Stop()
		}
		return e.Next()
	})
	return nil
}

func linkerInit(app core.App, r *core.Record) func() *Linker {
	return func() *Linker {
		ctx := context.Background()
		ctx, stop := context.WithCancel(ctx)
		linker := &Linker{
			app:  app,
			Stop: stop,
		}
		linker.SetProxyRecord(r)
		go linker.Start(ctx)
		return linker
	}
}

type Linker struct {
	core.BaseRecordProxy
	app  core.App
	Stop context.CancelFunc
}

func (lk *Linker) Start(ctx context.Context) {
	link := lk.GetString("linker")
	switch {
	case
		strings.HasPrefix(link, "http"),
		strings.HasPrefix(link, "ws"):
		lk.StartWS(ctx)
	case strings.HasPrefix(link, "ssh"):
		lk.StartSSH(ctx)
	}
}

func (lk *Linker) StartWS(ctx context.Context) {
	logger := lk.app.Logger().With(
		"id", lk.Id,
		"linker", lk.GetString("linker"),
	)
	retry.Do(func() (err error) {
		defer err0.Then(&err, nil, func() {
			logger.Error("wshttp 连接出错", "error", err)
		})

		lk.updateStatus("connecting")

		link := lk.GetString("linker")
		u := try.To1(url.Parse(link))

		opts := &websocket.DialOptions{
			Subprotocols: []string{"wshttp"},
		}
		if uinfo := u.User; uinfo != nil {
			u.User = nil // 移除 User
			p := opts.Subprotocols
			if uname := uinfo.Username(); uname != "" {
				p = append(p, uname)
				if pass, _ := uinfo.Password(); pass != "" {
					p = append(p, pass)
				}
			}
			opts.Subprotocols = p
		}

		u.Fragment = "" //要去除 Fragment
		link = u.String()

		socket, _ := try.To2(websocket.Dial(ctx, link, opts))
		conn := websocket.NetConn(ctx, socket, websocket.MessageBinary)
		sess := try.To1(yamux.Server(conn, nil))
		defer sess.Close()

		try.To1(sess.Ping())

		lk.updateStatus("connected")
		return http.Serve(sess, wgHandler)
	},
		retry.Context(ctx),
		retry.Attempts(0),
		retry.MaxDelay(15*time.Second),
	)
	lk.updateStatus("stopped")
}

// ssh://user:pass@sshd.host:22/80/127.0.0.1
func (lk *Linker) StartSSH(ctx context.Context) {
	logger := lk.app.Logger().With(
		"id", lk.Id,
		"linker", lk.GetString("linker"),
	)
	retry.Do(func() (err error) {
		defer err0.Then(&err, nil, func() {
			logger.Error("ssh 连接出错", "error", err)
		})

		lk.updateStatus("connecting")

		link := lk.GetString("linker")
		u := try.To1(url.Parse(link))

		user := u.User.Username()
		pass, _ := u.User.Password()
		port := u.Port()
		if port == "" {
			port = "22"
		}
		addr := net.JoinHostPort(u.Hostname(), port)

		config := &ssh.ClientConfig{
			User:            user,
			Auth:            []ssh.AuthMethod{ssh.Password(pass)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			BannerCallback:  func(message string) error { return nil },
		}
		client := try.To1(ssh.Dial("tcp", addr, config))
		defer client.Close()

		var (
			rhost = "127.0.0.1"
			rport = "80"
		)
		paths := strings.SplitN(u.Path, "/", 4)
		if len(paths) >= 2 {
			rport = paths[1]
		}
		if len(paths) >= 3 {
			rhost = paths[2]
		}

		raddr := net.JoinHostPort(rhost, rport)
		ln := try.To1(client.Listen("tcp", raddr))
		defer ln.Close()

		lk.updateStatus("connected")
		return http.Serve(ln, wgHandler)
	},
		retry.Context(ctx),
		retry.Attempts(0),
		retry.MaxDelay(15*time.Second),
	)
	lk.updateStatus("stopped")
}

func (lk *Linker) updateStatus(s string) {
	q := dbx.HashExp{"id": lk.Id}
	p := dbx.Params{"status": s}
	logger := lk.app.Logger().With("id", lk.Id)
	if _, err := lk.app.DB().Update(db.TableLinkers, p, q).Execute(); err != nil {
		logger.Error("update linker status failed", "error", err)
	}
}
