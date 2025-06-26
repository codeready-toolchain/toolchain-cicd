package govulncheck

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseReport(t *testing.T) {
	t.Run("valid report", func(t *testing.T) {
		expectedOSVs := map[string]*OSV{
			"GO-2024-2611": {
				ID:      "GO-2024-2611",
				Summary: "Infinite loop in JSON unmarshaling in google.golang.org/protobuf",
				DatabaseSpecific: DatabaseSpecific{
					URL: "https://pkg.go.dev/vuln/GO-2024-2611",
				},
			},
			"GO-2025-3563": {
				ID:      "GO-2025-3563",
				Summary: "Request smuggling due to acceptance of invalid chunked data in net/http",
				DatabaseSpecific: DatabaseSpecific{
					URL: "https://pkg.go.dev/vuln/GO-2025-3563",
				},
			},
			"GO-2025-3547": {
				ID:      "GO-2025-3547",
				Summary: "Kubernetes kube-apiserver Vulnerable to Race Condition in k8s.io/kubernetes",
				DatabaseSpecific: DatabaseSpecific{
					URL: "https://pkg.go.dev/vuln/GO-2025-3547",
				},
			},
		}

		expectedFindings := map[string][]*Finding{
			"GO-2025-3563": {
				{
					Osv:          "GO-2025-3563",
					FixedVersion: "v1.23.8",
					Trace: []Trace{
						{
							Module:   "stdlib",
							Version:  "v1.22.12",
							Package:  "net/http/internal",
							Function: "Read",
							Position: Position{
								Filename: "src/net/http/internal/chunked.go",
								Line:     97,
								Column:   26,
							},
						},
						{
							Module:   "stdlib",
							Version:  "v1.22.12",
							Package:  "net/http",
							Function: "readLocked",
							Position: Position{
								Filename: "src/net/http/transfer.go",
								Line:     840,
								Column:   21,
							},
						},
						{
							Module:   "package",
							Package:  "package/pkg/configuration",
							Function: "Load",
							Position: Position{
								Filename: "pkg/configuration/config.go",
								Line:     95,
								Column:   26,
							},
						},
					},
				},
			},
			"GO-2025-3547": {
				{
					Osv: "GO-2025-3547",
					Trace: []Trace{
						{
							Module:   "k8s.io/kubernetes",
							Version:  "v1.30.10",
							Package:  "k8s.io/kubernetes/pkg/features",
							Function: "init",
							Position: Position{
								Filename: "pkg/features/client_adapter.go",
								Line:     17,
								Column:   1,
							},
						},
						{
							Module:   "package",
							Package:  "package",
							Function: "init",
							Position: Position{
								Filename: "main.go",
								Line:     46,
								Column:   2,
							},
						},
					},
				},
				{
					Osv: "GO-2025-3547",
					Trace: []Trace{
						{
							Module:   "k8s.io/kubernetes",
							Version:  "v1.30.10",
							Package:  "k8s.io/kubernetes/pkg/kubelet/cri/remote",
							Function: "ContainerStatus",
							Position: Position{
								Filename: "pkg/kubelet/cri/remote/remote_runtime.go",
								Line:     416,
								Column:   32,
							},
						},
						{
							Module:   "package",
							Package:  "package/pkg/cri",
							Function: "GetContainersPerPID",
							Position: Position{
								Filename: "pkg/cri/containers.go",
								Line:     39,
								Column:   52,
							},
						},
					},
				},
			},
		}

		// given
		report, err := os.ReadFile("../testdata/valid_report.json")
		require.NoError(t, err)
		// when
		parsedReport, err := parseReport(report)
		require.NoError(t, err)
		// then
		assert.Len(t, parsedReport.Finding, 2)
		assert.Equal(t, expectedFindings, parsedReport.Finding)
		assert.Len(t, parsedReport.OSV, 3)
		assert.Equal(t, expectedOSVs, parsedReport.OSV)
	})

	t.Run("invalid report - error decoding JSON", func(t *testing.T) {
		// given
		report := []byte(`{`)
		// when
		_, err := parseReport(report)
		// then
		require.EqualError(t, err, "error decoding JSON: unexpected EOF")
	})

	t.Run("invalid report - failed to unmarshal Finding struct", func(t *testing.T) {
		// given
		report := []byte(`
{
  "finding": {
    "osv": "GO-2025-3563",
    "fixed_version": "v1.23.8",
    "trace": [
      {
        "module": "stdlib",
        "version": 1
      }
    ]
  }
}
`)
		// when
		_, err := parseReport(report)
		// then
		require.EqualError(t, err, "failed to unmarshal Finding struct: json: cannot unmarshal number into Go struct field Trace.trace.version of type string")
	})
}
