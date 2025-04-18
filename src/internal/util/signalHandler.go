package util

type SignalHandlers struct {
	Sighup  func()
	Sigint  func()
	Sigterm func()

	OnHandlerPanic func(any)
}

func tryCall(fun func(), onPanic func(any)) {
	defer func() {
		if onPanic != nil {
			if err := recover(); err != nil {
				onPanic(err)
			}
		}
	}()
	if fun != nil {
		fun()
	}
}
