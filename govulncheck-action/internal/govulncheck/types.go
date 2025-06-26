package govulncheck

type Trace struct {
	Module   string   `json:"module"`
	Version  string   `json:"version"`
	Package  string   `json:"package"`
	Function string   `json:"function"`
	Position Position `json:"position"`
}

type Position struct {
	Filename string `json:"filename"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
}

type Finding struct {
	Osv          string  `json:"osv"`
	FixedVersion string  `json:"fixed_version"`
	Trace        []Trace `json:"trace"`
}

type DatabaseSpecific struct {
	URL string `json:"url"`
}
type OSV struct {
	ID               string           `json:"id"`
	Summary          string           `json:"summary"`
	DatabaseSpecific DatabaseSpecific `json:"database_specific"`
}

type Report struct {
	// 1* findings per vuln
	Finding map[string][]*Finding `json:"finding,omitempty"`
	// 1 OSV per vuln
	OSV map[string]*OSV `json:"osv,omitempty"`
}

type Vulnerability struct {
	ID       string
	Summary  string
	MoreInfo string
	FoundIn  string
	FixedIn  string
	Traces   []string
}
