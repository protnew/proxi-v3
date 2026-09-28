// Package archive holds frozen historical sources excluded from builds via //go:build ignore.
// Do not add active code here. Rename archived *_test.go out of the package or tag ignore
// so they cannot duplicate Test* symbols with live packages (e.g. api_test-v1/v2).
package archive
