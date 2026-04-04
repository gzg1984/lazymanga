package rulebook

import "strings"

func boolPtr(v bool) *bool {
	b := v
	return &b
}

type defaultOSKeywordRule struct {
	TypeName  string
	TargetDir string
	Keywords  []string
}

var defaultOSKeywordRules = []defaultOSKeywordRule{
	{TypeName: "Ubuntu", TargetDir: "OS/Linux/Ubuntu", Keywords: []string{"ubuntu", "kubuntu", "xubuntu", "lubuntu"}},
	{TypeName: "Debian", TargetDir: "OS/Linux/Debian", Keywords: []string{"debian"}},
	{TypeName: "Fedora", TargetDir: "OS/Linux/Fedora", Keywords: []string{"fedora"}},
	{TypeName: "CentOS", TargetDir: "OS/Linux/CentOS", Keywords: []string{"centos"}},
	{TypeName: "RHEL", TargetDir: "OS/Linux/RHEL", Keywords: []string{"rhel", "redhat", "red-hat"}},
	{TypeName: "Rocky Linux", TargetDir: "OS/Linux/Rocky", Keywords: []string{"rocky"}},
	{TypeName: "AlmaLinux", TargetDir: "OS/Linux/AlmaLinux", Keywords: []string{"almalinux", "alma"}},
	{TypeName: "SLE", TargetDir: "OS/Linux/SUSELinuxEnterprise", Keywords: []string{"sles", "sle"}},
	{TypeName: "OpenSUSE", TargetDir: "OS/Linux/OpenSUSE", Keywords: []string{"opensuse", "open-suse", "suse"}},
	{TypeName: "Arch Linux", TargetDir: "OS/Linux/Arch", Keywords: []string{"archlinux", "arch"}},
	{TypeName: "Manjaro", TargetDir: "OS/Linux/Manjaro", Keywords: []string{"manjaro"}},
	{TypeName: "Kali Linux", TargetDir: "OS/Linux/Kali", Keywords: []string{"kali"}},
	{TypeName: "Linux Mint", TargetDir: "OS/Linux/Mint", Keywords: []string{"linuxmint", "mint"}},
	{TypeName: "Windows 11", TargetDir: "OS/Windows/Windows11", Keywords: []string{"windows11", "windows-11", "win11"}},
	{TypeName: "Windows 10", TargetDir: "OS/Windows/Windows10", Keywords: []string{"windows10", "windows-10", "win10"}},
	{TypeName: "Windows", TargetDir: "OS/Windows", Keywords: []string{"windows"}},
	{TypeName: "macOS", TargetDir: "OS/macOS", Keywords: []string{"macos", "osx", "ventura", "sonoma", "sequoia"}},
	{TypeName: "VMware ESXi", TargetDir: "OS/VMware/ESXi", Keywords: []string{"esxi", "vmvisor"}},
}

// DefaultNoopRuleBook returns a rule book that performs no relocation actions.
func DefaultNoopRuleBook() RuleBook {
	return RuleBook{Name: "noop", Version: "v1", Rules: []Rule{}}
}

// DefaultOSRelocationRuleBook returns the compatibility rule book for current relocation behavior.
func DefaultOSRelocationRuleBook() RuleBook {
	rules := make([]Rule, 0, 2+len(defaultOSKeywordRules)*2)
	priority := 10

	rules = append(rules, Rule{
		ID:       "entertainment-explicit",
		Priority: priority,
		Enabled:  true,
		Match: Condition{
			IsEntertainment: boolPtr(true),
		},
		Action: Action{TargetDir: "Entertainment", RuleType: "Entertainment"},
	})
	priority += 10

	for _, r := range defaultOSKeywordRules {
		rules = append(rules, Rule{
			ID:       "os-explicit-" + sanitizeRuleID(r.TypeName),
			Priority: priority,
			Enabled:  true,
			Match: Condition{
				IsOS:            boolPtr(true),
				IsEntertainment: boolPtr(false),
				FileNameContains: r.Keywords,
			},
			Action: Action{TargetDir: r.TargetDir, RuleType: r.TypeName},
		})
		priority += 10
	}

	rules = append(rules, Rule{
		ID:       "os-explicit-fallback",
		Priority: priority,
		Enabled:  true,
		Match: Condition{
			IsOS:            boolPtr(true),
			IsEntertainment: boolPtr(false),
		},
		Action: Action{TargetDir: "OS", RuleType: "OS"},
	})
	priority += 10

	for _, r := range defaultOSKeywordRules {
		rules = append(rules, Rule{
			ID:       "os-infer-" + sanitizeRuleID(r.TypeName),
			Priority: priority,
			Enabled:  true,
			Match: Condition{
				IsOS:            boolPtr(false),
				IsEntertainment: boolPtr(false),
				FileNameContains: r.Keywords,
			},
			Action: Action{TargetDir: r.TargetDir, RuleType: r.TypeName, InferIsOS: true},
		})
		priority += 10
	}

	return RuleBook{Name: "default-os-relocation", Version: "v1", Rules: rules}
}

func sanitizeRuleID(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	v = strings.ReplaceAll(v, " ", "-")
	return strings.ReplaceAll(v, "/", "-")
}
