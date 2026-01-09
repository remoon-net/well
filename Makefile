VERSION := $(shell git describe --tags --always --dirty)

ui:
	cd ../well-webui/ && go generate .
build: 
	CGO_ENABLED=0 go build  -ldflags="-X 'remoon.net/well/cmd.Version=${VERSION}' -s -w" -o well-net .
deb: ui build
	nfpm pkg --packager deb
aar: ui
	gomobile bind -o ../well-android/app/libs/well-net.aar -ldflags "-X 'remoon.net/well/cmd.Version=${VERSION}' -checklinkname=0" -androidapi 21 -target=android -javapkg net.remoon.well ./cmd
exe:
	go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo -o versioninfo.syso
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-X 'main.Version=${VERSION}' -s -w" -o nsi/well-net.exe .
exe-installer: exe
	echo '!define VERSION "$(VERSION)"' > nsi/version.nsh
	cd nsi && makensis installer.nsi
