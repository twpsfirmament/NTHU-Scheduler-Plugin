package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

type CustomSchedulerArgs struct {
	Mode string `json:"mode"`
}

type CustomScheduler struct {
	handle    framework.Handle
	scoreMode string
}

var _ framework.PreFilterPlugin = &CustomScheduler{}
var _ framework.ScorePlugin = &CustomScheduler{}

// Name is the name of the plugin used in Registry and configurations.
const (
	Name              string = "CustomScheduler"
	groupNameLabel    string = "podGroup"
	minAvailableLabel string = "minAvailable"
	leastMode         string = "Least"
	mostMode          string = "Most"
)

func (cs *CustomScheduler) Name() string {
	return Name
}

// New initializes and returns a new CustomScheduler plugin.
func New(obj runtime.Object, h framework.Handle) (framework.Plugin, error) {
	cs := CustomScheduler{}
	mode := leastMode
	if obj != nil {
		args := obj.(*runtime.Unknown)
		var csArgs CustomSchedulerArgs
		if err := json.Unmarshal(args.Raw, &csArgs); err != nil {
			fmt.Printf("Error unmarshal: %v\n", err)
		}
		mode = csArgs.Mode
		if mode != leastMode && mode != mostMode {
			return nil, fmt.Errorf("invalid mode, got %s", mode)
		}
	}
	cs.handle = h
	cs.scoreMode = mode
	log.Printf("Custom scheduler runs with the mode: %s.", mode)

	return &cs, nil
}

// filter the pod if the pod in group is less than minAvailable
func (cs *CustomScheduler) PreFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod) (*framework.PreFilterResult, *framework.Status) {
	log.Printf("Pod %s is in Prefilter phase.", pod.Name)
	newStatus := framework.NewStatus(framework.Success, "")

	// TODO
	// 1. extract the label of the pod
	// 2. retrieve the pod with the same group label
	// 3. justify if the pod can be scheduled

	label, exists := pod.ObjectMeta.Labels[groupNameLabel]
	minAvailable := pod.ObjectMeta.Labels[minAvailableLabel]
	if !exists {
		return nil, framework.NewStatus(framework.Success, "no group label")
	}

	selector := labels.SelectorFromSet(labels.Set{groupNameLabel: label})

	pods, err := cs.handle.SharedInformerFactory().Core().V1().Pods().Lister().Pods(pod.Namespace).List(selector)
	if err != nil {
		return nil, framework.NewStatus(framework.Error, err.Error())
	}

	minAvailableInt, _ := strconv.Atoi(minAvailable)
	if len(pods) < minAvailableInt {
		return nil, framework.NewStatus(framework.Unschedulable, "not enough pods in the group")
	}

	// return the result?
	return nil, newStatus
}

// PreFilterExtensions returns a PreFilterExtensions interface if the plugin implements one.
func (cs *CustomScheduler) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

// Score invoked at the score extension point.
func (cs *CustomScheduler) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
	log.Printf("Pod %s is in Score phase. Calculate the score of Node %s.", pod.Name, nodeName)

	// TODO
	// 1. retrieve the node allocatable memory
	nodeInfo, err := cs.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, err.Error())
	}
	nodeMemory := nodeInfo.Node().Status.Allocatable.Memory().Value()
	// 2. return the score based on the scheduler mode
	if cs.scoreMode == leastMode {
		return -nodeMemory, nil
	} else if cs.scoreMode == mostMode {
		return nodeMemory, nil
	}
	return 0, nil
}

// ensure the scores are within the valid range
func (cs *CustomScheduler) NormalizeScore(ctx context.Context, state *framework.CycleState, pod *v1.Pod, scores framework.NodeScoreList) *framework.Status {
	// TODO
	// find the range of the current score and map to the valid range

	if len(scores) == 0 {
		return framework.NewStatus(framework.Success, "No scores to normalize")
	}

	// Find the min and max scores
	minScore := scores[0].Score
	maxScore := scores[0].Score
	for _, nodeScore := range scores {
		if nodeScore.Score < minScore {
			minScore = nodeScore.Score
		}
		if nodeScore.Score > maxScore {
			maxScore = nodeScore.Score
		}
	}

	// Define the valid range
	validMin := int64(0)
	validMax := int64(framework.MaxNodeScore)

	// If all scores are the same, map them to the middle of the range
	if minScore == maxScore {
		for i := range scores {
			scores[i].Score = (validMin + validMax) / 2
		}
		return framework.NewStatus(framework.Success, "")
	}

	// Normalize the scores to the valid range
	scoreRange := maxScore - minScore
	validRange := validMax - validMin
	for i := range scores {
		scores[i].Score = ((scores[i].Score - minScore) * validRange / scoreRange) + validMin
	}

	return framework.NewStatus(framework.Success, "")
	return nil
}

// ScoreExtensions of the Score plugin.
func (cs *CustomScheduler) ScoreExtensions() framework.ScoreExtensions {
	return cs
}
