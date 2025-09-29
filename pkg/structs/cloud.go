package structs

type Machine struct {
	ID               string
	Name             string
	OrganizationInfo map[string]string `json:"organization_info"`
	Dimensions       map[string]string
	Age              string
	CreatedAtRfc3339 string `json:"created_at_rfc3339"`
}

type Machines []Machine
