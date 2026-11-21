package config

import "gopkg.in/yaml.v3"

// Workflow is one GitHub Actions workflow file as the wiring rules read it: the
// events that start it, the branches a push names, and its steps in job order.
type Workflow struct {
	Triggers     map[string]bool
	PushBranches []string
	Steps        []WorkflowStep
}

// WorkflowStep is one job step: its run text, the line that text starts on, the
// line of the step itself, and whether a failure of the step is tolerated.
type WorkflowStep struct {
	Line            int
	Run             string
	RunLine         int
	ContinueOnError bool
}

// ParseWorkflow reads one workflow file's text. A file that is not YAML is an
// error; a file with no jobs or no events reads as an empty workflow.
func ParseWorkflow(text string) (*Workflow, error) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(text), &root); err != nil {
		return nil, err
	}
	workflow := &Workflow{Triggers: map[string]bool{}}
	if len(root.Content) == 0 {
		return workflow, nil
	}
	document := root.Content[0]
	decodeTriggers(workflow, mappingValue(document, "on"))
	jobs := mappingValue(document, "jobs")
	if jobs == nil || jobs.Kind != yaml.MappingNode {
		return workflow, nil
	}
	for index := 1; index < len(jobs.Content); index += 2 {
		steps := mappingValue(jobs.Content[index], "steps")
		if steps == nil || steps.Kind != yaml.SequenceNode {
			continue
		}
		for _, step := range steps.Content {
			workflow.Steps = append(workflow.Steps, decodeStep(step))
		}
	}
	return workflow, nil
}

// decodeTriggers reads the `on` key in its three shapes: one event, a list of
// events, or a mapping of events, where a push may name branches.
func decodeTriggers(workflow *Workflow, on *yaml.Node) {
	if on == nil {
		return
	}
	switch on.Kind {
	case yaml.ScalarNode:
		workflow.Triggers[on.Value] = true
	case yaml.SequenceNode:
		for _, item := range on.Content {
			workflow.Triggers[item.Value] = true
		}
	case yaml.MappingNode:
		for index := 0; index < len(on.Content); index += 2 {
			event := on.Content[index].Value
			workflow.Triggers[event] = true
			if event != "push" {
				continue
			}
			if branches := mappingValue(on.Content[index+1], "branches"); branches != nil && branches.Kind == yaml.SequenceNode {
				for _, branch := range branches.Content {
					workflow.PushBranches = append(workflow.PushBranches, branch.Value)
				}
			}
		}
	}
}

// decodeStep reads one step's run text, where that text starts, and its
// continue-on-error flag. A block scalar's text starts on the line after its
// indicator.
func decodeStep(step *yaml.Node) WorkflowStep {
	decoded := WorkflowStep{Line: step.Line}
	if run := mappingValue(step, "run"); run != nil && run.Kind == yaml.ScalarNode {
		decoded.Run = run.Value
		decoded.RunLine = run.Line
		if run.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
			decoded.RunLine++
		}
	}
	if continued := mappingValue(step, "continue-on-error"); continued != nil && continued.Value == "true" {
		decoded.ContinueOnError = true
	}
	return decoded
}
