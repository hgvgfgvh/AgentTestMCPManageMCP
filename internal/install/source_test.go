package install

import "testing"

func TestEnrichPlan_NPM(t *testing.T) {
	p := EnrichPlan(Plan{Requirement: `安装 npx -y @modelcontextprotocol/server-filesystem D:\data`})
	if p.SourceKind != SourceNPM {
		t.Fatalf("source=%q", p.SourceKind)
	}
	if p.NPMPackage != "@modelcontextprotocol/server-filesystem" {
		t.Fatalf("package=%q", p.NPMPackage)
	}
}

func TestEnrichPlan_JSON(t *testing.T) {
	p := EnrichPlan(Plan{Requirement: `{"source":"npm","package":"@scope/pkg","summary":"测试"}`})
	if p.SourceKind != SourceNPM || p.NPMPackage != "@scope/pkg" {
		t.Fatalf("%+v", p)
	}
}

func TestEnrichPlan_HTTP(t *testing.T) {
	p := EnrichPlan(Plan{Requirement: "从 https://example.com/pkg.zip 安装"})
	if p.SourceKind != SourceHTTP || p.URL == "" {
		t.Fatalf("%+v", p)
	}
}
