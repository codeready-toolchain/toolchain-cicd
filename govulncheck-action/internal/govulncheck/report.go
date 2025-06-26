package govulncheck

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// hasVulnerability determines if the finding reports a vulnerability
// we can know it by checking if function field is present
// example:
//
//	"finding": {
//	    "osv": "GO-2025-3547",
//	    "trace": [
//	      {
//	        "module": "k8s.io/kubernetes",
//	        "version": "v1.30.10",
//	        "package": "k8s.io/kubernetes/pkg/kubelet/cri/remote",
//	        "function": "ListContainers",
//	        "receiver": "*remoteRuntimeService",
//	        "position": {
//	          "filename": "pkg/kubelet/cri/remote/remote_runtime.go",
//	          "offset": 14659,
//	          "line": 394,
//	          "column": 32
//	        }
//	      },
func hasVulnerability(trace []Trace) bool {
	for _, t := range trace {
		if t.Function != "" {
			return true
		}
	}
	return false
}

func parseReport(rawReport []byte) (*Report, error) {
	report := &Report{
		Finding: make(map[string][]*Finding),
		OSV:     make(map[string]*OSV),
	}

	decoder := json.NewDecoder(bytes.NewReader(rawReport))

	for {
		var obj map[string]interface{}

		if err := decoder.Decode(&obj); err != nil {
			if err.Error() == "EOF" {
				// stop the loop when we get into the end of the file
				break
			}
			return nil, fmt.Errorf("error decoding JSON: %w", err)
		}

		// save all findings (results)
		if finding, ok := obj["finding"]; ok {
			var f Finding
			findingBytes, err := json.Marshal(finding)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal to Finding struct: %w", err)
			}
			if err := json.Unmarshal(findingBytes, &f); err != nil {
				return &Report{}, fmt.Errorf("failed to unmarshal Finding struct: %w", err)
			}
			if hasVulnerability(f.Trace) {
				report.Finding[f.Osv] = append(report.Finding[f.Osv], &f)
			}

			// save all osv entries (rules)
		} else if osv, ok := obj["osv"]; ok {
			var o OSV
			osvBytes, err := json.Marshal(osv)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal to OSV struct: %w", err)
			}
			if err := json.Unmarshal(osvBytes, &o); err != nil {
				return &Report{}, fmt.Errorf("failed to unmarshal OSV struct: %w", err)
			}
			report.OSV[o.ID] = &o
		}

	}

	return report, nil
}
