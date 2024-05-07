package label

const (
	LabelInstanceKey        = "finetune.datatunerx.io/instance"
	LabelDatatunerx         = "datatunerx"
	LabelFinetuneJob        = "finetunejob"
	LabelFinetune           = "finetune"
	LabelFinetuneExperiment = "finetuneexperiment"
	LabelComponentKey       = "finetune.datatunerx.io/component"
	LabelPartOfKey          = "finetune.datatunerx.io/part-of"
	LabelFinetuneBindingKey = "finetune.datatunerx.io/finetunebinding"
)

func GenerateInstanceLabel(instanceName string, customLabels map[string]string) map[string]string {
	if customLabels == nil {
		customLabels = make(map[string]string)
	}
	baseLabel := GetBaseLabel()
	customLabel := MergeLabel(map[string]string{
		LabelInstanceKey:  instanceName,
		LabelComponentKey: LabelFinetuneJob,
	}, customLabels)
	return MergeLabel(baseLabel, customLabel)
}

func GetBaseLabel() map[string]string {
	return map[string]string{
		LabelPartOfKey: LabelDatatunerx,
	}
}

func MergeLabel(baseLabel map[string]string, customLabel map[string]string) map[string]string {
	if baseLabel == nil {
		baseLabel = make(map[string]string)
	}
	for k, v := range customLabel {
		if _, exists := baseLabel[k]; !exists {
			baseLabel[k] = v
		}
	}
	return baseLabel
}
