package util

import (
	"os"

	log "github.com/cihub/seelog"
)

const (
	defaultLogConfig = `
<seelog type="sync" minlevel="info">
    <outputs formatid="main">
        <console />
    </outputs>
    <formats>
        <format id="main" format="%Date/%Time [%LEV] %Msg%n"/>
    </formats>
</seelog>
`

	testLogConfig = `
<seelog type="sync" minlevel="info">
    <outputs formatid="main">
        <console />
    </outputs>
    <formats>
        <format id="main" format="%Date/%Time [%LEV] %Msg%n"/>
    </formats>
</seelog>
`
)

func init() {
	logConfig := defaultLogConfig
	if os.Getenv("CI") == "true" {
		logConfig = testLogConfig
	}

	logger, err := log.LoggerFromConfigAsString(logConfig)
	if err != nil {
		panic(err)
	}
	log.ReplaceLogger(logger)
}
