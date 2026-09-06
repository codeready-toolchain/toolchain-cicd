package cmd

type Pull struct {
	Author  string `json:"author"`
	HeadRef string `json:"head_ref"`
}

type Refs struct {
	Pulls []Pull `json:"pulls"`
}

type JobSpec struct {
	Refs Refs `json:"refs"`
}
