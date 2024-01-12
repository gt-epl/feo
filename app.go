package main

import "strconv"

type Application struct {
	offloader OffloaderIntf
	portChan  chan string
}

func createApplication(appName string, initPortNumber, numReplicas int, offloader OffloaderIntf) *Application {
	portChannel := make(chan string, numReplicas)
	for i := 0; i < numReplicas; i++ {
		portChannel <- strconv.Itoa(initPortNumber + i)
	}

	app := &Application{
		offloader: offloader,
		portChan:  portChannel,
	}
	return app
}
