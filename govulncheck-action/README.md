# govulncheck-action

A custom govulncheck action that ignores vulnerabilities listed in the `.govulncheck.yaml` file. Each entry can include a `silence_until` field, which sets an expiration date for how long the vulnerability should be silenced.
Once the `silence_until` date has passed, the vulnerability will reappear in the results, prompting you to reassess it.

## the `.govulncheck.yaml` file structure:

```
ignored-vulnerabilities:
    # comment vulnerability information
    - id: <vulnerability-id>
      silence-until: <silence-until-date>
      info: <vulnerability-info-link>
```

As an example:
```
ignored-vulnerabilities:
    # Kubernetes kube-apiserver Vulnerable to Race Condition in k8s.io/kubernetes
    # More info: https://pkg.go.dev/vuln/GO-2025-3547
    # Module: k8s.io/kubernetes
    # Fixed in: N/A
    - id: GO-2025-3547
      silence-until: 2020-05-10
      info: https://pkg.go.dev/vuln/GO-2025-3547
```

## Best practices

- Before choosing to ignore a specific vulnerability, ensure that no fix or viable workaround is available.

- The `silence_until` field for ignoring a vulnerability should be set within a one-month time frame.


## How to use it

```
name: govulncheck
on:
  pull_request:
    branches:
      - master

jobs:
  govulncheck:
    name: govulncheck
    runs-on: ubuntu-latest

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Install Go
      uses: actions/setup-go@v5
      with:
        go-version-file: go.mod

    - name: Run govulncheck
      uses: xcoulon/govulncheck-action@main
      with:
        go-version-file: go.mod
        config: .govulncheck.yaml
```
