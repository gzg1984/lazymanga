package normalization

import "lazymanga/normalization/fieldsemantics"

type FieldSemanticRole = fieldsemantics.Role

func ResolveFieldSemanticRole(key string) FieldSemanticRole {
	return fieldsemantics.Resolve(key)
}

func ShouldIncludeFieldInAnalysisModel(key string) bool {
	return fieldsemantics.ShouldIncludeInAnalysisModel(key)
}

func ShouldCountFieldAsSemanticProposalSignal(key string) bool {
	return fieldsemantics.ShouldCountAsSemanticProposalSignal(key)
}

func IsContextAnchorField(key string) bool {
	return fieldsemantics.IsContextAnchor(key)
}

func IsTitleRelatedField(key string) bool {
	return fieldsemantics.IsTitleRelated(key)
}

func ContextAnchorFields() []string {
	return fieldsemantics.ContextAnchorFields()
}

func ShouldOnlyFillEmptyField(key string) bool {
	return fieldsemantics.ShouldOnlyFillEmpty(key)
}

func CanAutoApplyFieldValue(key string, existingValue string) bool {
	return fieldsemantics.CanAutoApplyValue(key, existingValue)
}

func ShouldIncludeFieldInTextAnalyzerHints(key string) bool {
	return fieldsemantics.ShouldIncludeInTextAnalyzerHints(key)
}

func ShouldIncludeFieldInProposalChanges(key string) bool {
	return fieldsemantics.ShouldIncludeInProposalChanges(key)
}
