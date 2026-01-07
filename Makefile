build:
	CGO_ENABLED=0 go build  -ldflags="-X 'remoon.net/well/cmd.Version=$$(git describe --tags --always --dirty)' -s -w" -o well-net .
ui:
	cd ../well-webui/ && go generate .
aar: ui
	gomobile bind -o ../well-net/app/libs/well-net.aar -ldflags "-X 'remoon.net/well/cmd.Version=$$(git describe --tags --always --dirty)' -checklinkname=0" -androidapi 21 -target=android -javapkg net.remoon.well ./cmd
