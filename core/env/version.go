/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package env

type Version struct {
	VersionNumber   string `json:"number,omitempty"`
	BuildCommitHash string `json:"build_hash,omitempty"`
	BuildDate       string `json:"build_date,omitempty"`
	BuildNumber     string `json:"build_number,omitempty"`
	EolDate         string `json:"eol_date,omitempty"`
}

func (env *Env) GetVersionInfo() Version {
	return Version{
		VersionNumber:   env.version,
		BuildCommitHash: env.commit,
		BuildDate:       env.buildDate,
		BuildNumber:     env.buildNumber,
		EolDate:         env.eolDate,
	}
}
