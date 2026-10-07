// Package buildinfo carries version metadata injected at build time:
//
//	go build -ldflags "-X github.com/andi-frame/lockedin/apps/server/internal/buildinfo.Version=$(git describe --always)"
package buildinfo

var Version = "dev"
