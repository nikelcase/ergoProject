package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/ergo-services/ergo"
	"github.com/ergo-services/ergo/etf"
	"github.com/ergo-services/ergo/gen"
	"github.com/ergo-services/ergo/lib"
	"github.com/ergo-services/ergo/node"
)

const appname = "ergoProject"

func main() {
	flag.Parse()

	fmt.Println("Start node " + appname + "@127.0.0.1")
	myNode, err := ergo.StartNode(appname+"@127.0.0.1", appname, node.Options{})
	if err != nil {
		panic(err)
	}

	appSup := CreateAppSup()

	sup, err := myNode.Spawn(appname+"AppSup", gen.ProcessOptions{}, appSup)
	if err != nil {
		panic(err)
	}
	fmt.Println("Started supervisor process", sup.Self())
	sup.Wait()
}

//-----------------------------------------------------------------------------appSup

func CreateAppSup() gen.SupervisorBehavior {
	return &appSup{}
}

type appSup struct {
	gen.Supervisor
}

func (as *appSup) Init(args ...etf.Term) (gen.SupervisorSpec, error) {
	spec := gen.SupervisorSpec{
		Name: "appSup",
		Children: []gen.SupervisorChildSpec{
			gen.SupervisorChildSpec{
				Name:  "producer",
				Child: CreateProducer(),
			},
			gen.SupervisorChildSpec{
				Name:  "consumerWorkersAppSup",
				Child: createConsumerWorksersSup(),
			},
		},
		Strategy: gen.SupervisorStrategy{
			Type:      gen.SupervisorStrategyOneForAll,
			Intensity: 5,
			Period:    5,
			Restart:   gen.SupervisorStrategyRestartTransient,
		},
	}
	return spec, nil
}

//-----------------------------------------------------------------------------producer

const (
	ProducerEvent gen.Event = "notify"
)

func CreateProducer() gen.ServerBehavior {
	return &producer{}
}

type ProducerEventMessage struct {
	e string
}

type producer struct {
	gen.Server
}

func (p *producer) Init(process *gen.ServerProcess, args ...etf.Term) error {
	if err := process.RegisterEvent(ProducerEvent, ProducerEventMessage{}); err != nil {
		lib.Warning("can't register event %q: %s", ProducerEvent, err)
	}
	fmt.Printf("process %s registered event %s\n", process.Self(), ProducerEvent)
	process.SendAfter(process.Self(), 1, time.Second)
	return nil
}

func (p *producer) HandleInfo(process *gen.ServerProcess, message etf.Term) gen.ServerStatus {
	n := message.(int)
	if n > 2 {
		return gen.ServerStatusStop
	}
	// sending message with delay 1 second
	process.SendAfter(process.Self(), n+1, time.Second)
	event := ProducerEventMessage{
		e: fmt.Sprintf("EVNT %d", n),
	}

	fmt.Printf("... producing event: %s\n", event)
	if err := process.SendEventMessage(ProducerEvent, event); err != nil {
		fmt.Println("can't send event:", err)
	}
	return gen.ServerStatusOK
}

func (p *producer) Terminate(process *gen.ServerProcess, reason string) {
	fmt.Printf("[%s] Terminating process with reason %q", process.Name(), reason)
}

//-----------------------------------------------------------------------------consumerWorkersSup

func createConsumerWorksersSup() gen.SupervisorBehavior {
	return &consumerWorkersSup{}
}

type consumerWorkersSup struct {
	gen.Supervisor
}

func (cw *consumerWorkersSup) Init(args ...etf.Term) (gen.SupervisorSpec, error) {
	spec := gen.SupervisorSpec{
		Name: "consumerWorkersSup",
		Children: []gen.SupervisorChildSpec{
			gen.SupervisorChildSpec{
				Name:  "cons01",
				Child: CreateConsumer(),
			},
			gen.SupervisorChildSpec{
				Name:  "cons02",
				Child: CreateConsumer(),
			},
			gen.SupervisorChildSpec{
				Name:  "cons03",
				Child: CreateConsumer(),
			},
		},
		Strategy: gen.SupervisorStrategy{
			Type:      gen.SupervisorStrategyOneForOne,
			Intensity: 5,
			Period:    5,
			Restart:   gen.SupervisorStrategyRestartTransient,
		},
	}
	return spec, nil
}

//-----------------------------------------------------------------------------consumer

func CreateConsumer() gen.ServerBehavior {
	return &consumer{}
}

type consumer struct {
	gen.Server
}

func (c *consumer) Init(process *gen.ServerProcess, args ...etf.Term) error {
	fmt.Printf("Started new process\n\tPid: %s\n\tName: %q\n\tParent: %s\n\tArgs:%#v\n",
		process.Self(),
		process.Name(),
		process.Parent().Self(),
		args)
	if err := process.MonitorEvent(ProducerEvent); err != nil {
		return err
	}
	return nil
}

func (c *consumer) HandleInfo(process *gen.ServerProcess, message etf.Term) gen.ServerStatus {
	switch message.(type) {
	case ProducerEventMessage:
		fmt.Printf("consumer %s got event: %s\r\n", process.Self(), message)
	case gen.MessageEventDown:
		fmt.Printf("%s producer has terminated\r\n", process.Self())
		return gen.ServerStatusStop
	default:
		fmt.Println("unknown message", message)
	}
	return gen.ServerStatusOK
}

func (c *consumer) Terminate(process *gen.ServerProcess, reason string) {
	fmt.Printf("[%s] Terminating process with reason %q", process.Name(), reason)
}
