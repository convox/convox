package structs

const (
	AppParamBuildLabels = "BuildLabels"
	AppParamBuildCpu    = "BuildCpu"
	AppParamBuildMem    = "BuildMem"
)

type App struct {
	Generation string `json:"generation,omitempty"`
	Locked     bool   `json:"locked"`
	Name       string `json:"name"`
	Release    string `json:"release"`
	Router     string `json:"router"`
	Status     string `json:"status"`

	Outputs    map[string]string `json:"-"`
	Parameters map[string]string `json:"parameters"`
	Tags       map[string]string `json:"-"`
}

type Apps []App

type AppCreateOptions struct {
	Generation *string `default:"2" flag:"generation,g" param:"generation"`
	Timeout    *int    `flag:"timeout" param:"timeout"`
}

type AppUpdateOptions struct {
	Lock       *bool             `param:"lock"`
	Parameters map[string]string `param:"parameters"`
}

func (a Apps) Less(i, j int) bool {
	return a[i].Name < a[j].Name
}

type AppConfig struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
