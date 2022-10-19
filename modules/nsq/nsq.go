package nsq

import (
	log "github.com/cihub/seelog"
	"github.com/nsqio/nsq/nsqd"
	"infini.sh/framework/core/config"
)

type NSQModule struct {
}

func (module NSQModule) Name() string {
	return "NSQ"
}

func (module NSQModule) Setup() {

}

var done chan bool = make(chan bool)
var instance *nsqd.NSQD

func (module NSQModule) Start() error {

	// Run the embedded nsqd in a go routine
	go func() {
		// running an nsqd with all of the default options
		// (as if you ran it from the command line with no flags)
		// is literally these three lines of code. the nsqd
		// binary mainly wraps up the handling of command
		// line args and does something similar

		opts := nsqd.NewOptions()
		instance = nsqd.New(opts)
		instance.Main()

		log.Debug("starting nsq service")

		// wait until we are told to continue and exit
		<-done
	}()

	return nil
}

func (module NSQModule) Stop() error {
	// tell the nsqd instance to exit
	instance.Exit()
	done <- true
	return nil
}
