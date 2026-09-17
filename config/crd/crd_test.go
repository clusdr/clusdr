package crd

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestClusdrClusterCRDShape(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("clusdr.io_clusdrclusters.yaml")
	if err != nil {
		t.Fatal(err)
	}
	raw := string(b)
	for _, want := range []string{
		"clusdrclusters.clusdr.io",
		"group: clusdr.io",
		"kind: ClusdrCluster",
		"voterCount",
		"topology",
		"dataDir",
		"leave",
	} {
		if !strings.Contains(raw, want) {
			t.Errorf("CRD missing %q", want)
		}
	}
	for _, no := range []string{
		"kind: ClusdrMember",
		"ClusdrMemberList",
	} {
		if strings.Contains(raw, no) {
			t.Errorf("CRD must not mention %q", no)
		}
	}

	var doc map[string]any
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["kind"] != "CustomResourceDefinition" {
		t.Fatalf("kind: %v", doc["kind"])
	}
	spec, _ := doc["spec"].(map[string]any)
	if spec["group"] != "clusdr.io" {
		t.Fatalf("group: %v", spec["group"])
	}
	names, _ := spec["names"].(map[string]any)
	if names["kind"] != "ClusdrCluster" {
		t.Fatalf("names.kind: %v", names["kind"])
	}
	if spec["scope"] != "Namespaced" {
		t.Fatalf("scope: %v", spec["scope"])
	}

	props := crdSpecProperties(t, spec)
	for _, no := range []string{"clusterIP", "endpoint", "service", "members", "grpcAddr"} {
		if _, ok := props[no]; ok {
			t.Errorf("spec.properties must not include %q (not topology)", no)
		}
	}
	for _, want := range []string{"topology", "voterCount", "image", "dataDir", "leave"} {
		if _, ok := props[want]; !ok {
			t.Errorf("spec.properties missing %q", want)
		}
	}
}

func crdSpecProperties(t *testing.T, spec map[string]any) map[string]any {
	t.Helper()
	versions, _ := spec["versions"].([]any)
	if len(versions) == 0 {
		t.Fatal("no versions")
	}
	v0, _ := versions[0].(map[string]any)
	schema, _ := v0["schema"].(map[string]any)
	openAPI, _ := schema["openAPIV3Schema"].(map[string]any)
	objProps, _ := openAPI["properties"].(map[string]any)
	specObj, _ := objProps["spec"].(map[string]any)
	props, _ := specObj["properties"].(map[string]any)
	if props == nil {
		t.Fatal("spec.properties missing")
	}
	return props
}
