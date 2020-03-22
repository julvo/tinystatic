all: dist/tinystatic_macos_darwin_amd64 dist/tinystatic_linux_amd64

dist/tinystatic_macos_darwin_amd64: *.go
	GOOS=darwin GOARCH=amd64 go build -o dist/tinystatic_macos_darwin_amd64

dist/tinystatic_linux_amd64: *.go
	GOOS=linux GOARCH=amd64 go build -o dist/tinystatic_linux_amd64

