package main

func main() {
	CommonMain(CommonSetup())
}

// g build pic.go common.go
// sudo setcap cap_net_bind_service=+ep ./pic
