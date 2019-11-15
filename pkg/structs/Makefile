.PHONY: mocks

mocks:
	rm -f mock_Provider.go
	go run ../../vendor/github.com/vektra/mockery/cmd/mockery/mockery.go -inpkg -name Provider
