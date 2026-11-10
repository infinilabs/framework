package pipeline

import (
	"context"
	"fmt"
	"testing"

	corepipe "infini.sh/framework/core/pipeline"
)

// The observer and the query source must be ONE flight recorder — two
// instances filed real processor steps where flightQuery could never
// read them (PR #433 review).
func TestFlightObserverSeesRealSteps(t *testing.T) {
	installFlight()
	chain := &corepipe.Processors{List: []corepipe.Processor{pr433FailProcessor{}}}
	c := &corepipe.Context{Context: context.Background(), Config: corepipe.PipelineConfigV2{Name: t.Name()}}
	chain.Process(c)
	if got := flightQuery(t.Name(), "", true, 10); len(got) == 0 {
		t.Fatal("real failed processor step never reached flightQuery")
	}
}

type pr433VerifyFail struct{}

func (pr433VerifyFail) Name() string                    { return "pr433_verify_fail" }
func (pr433VerifyFail) Process(*corepipe.Context) error { return fmt.Errorf("fail") }

func TestActualProcessorChainReachesFlightQuery(t *testing.T) {
	installFlight()
	c := &corepipe.Context{Context: context.Background(), Config: corepipe.PipelineConfigV2{Name: t.Name()}}
	err := (&corepipe.Processors{List: []corepipe.Processor{pr433VerifyFail{}}}).Process(c)
	if err == nil {
		t.Fatal("failed processor did not run")
	}
	if got := flightQuery(t.Name(), "", true, 10); len(got) == 0 {
		t.Fatal("error event emitted by the actual processor chain missing from the query")
	}
}
