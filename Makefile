build:
	CGO_ENABLED=0 go build  -ldflags="-X 'remoon.net/well/cmd.Version=$$(git describe --tags --always --dirty)' -s -w" -o well-net .
aar:
	gomobile bind -o pb_data/well-net.aar -ldflags "-X 'remoon.net/well/cmd.Version=$$(git describe --tags --always --dirty)' -checklinkname=0" -androidapi 21 -target=android ./cmd
