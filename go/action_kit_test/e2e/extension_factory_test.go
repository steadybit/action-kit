package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func pod(name string, ready bool, deleting bool) corev1.Pod {
	p := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: status}}
	if deleting {
		now := metav1.Now()
		p.DeletionTimestamp = &now
	}
	return p
}

func TestActivePods_ignores_terminating_pod_of_previous_revision(t *testing.T) {
	pods, key, allReady := activePods([]corev1.Pod{pod("old", false, true), pod("new", true, false)})

	assert.Len(t, pods, 1)
	assert.Equal(t, "new", pods[0].Name)
	assert.Equal(t, "new", key)
	assert.True(t, allReady)
}

func TestActivePods_not_ready_while_a_pod_is_not_ready(t *testing.T) {
	_, key, allReady := activePods([]corev1.Pod{pod("b", true, false), pod("a", false, false)})

	assert.Equal(t, "a,b", key, "the key is independent of list order")
	assert.False(t, allReady)
}

func TestActivePods_key_changes_when_previous_revision_starts_terminating(t *testing.T) {
	_, before, _ := activePods([]corev1.Pod{pod("old", true, false), pod("new", true, false)})
	_, after, _ := activePods([]corev1.Pod{pod("old", true, true), pod("new", true, false)})

	assert.NotEqual(t, before, after, "a set that is still changing must not be considered stable")
}

func TestIsPodReady_requires_running_phase(t *testing.T) {
	p := pod("pending", true, false)
	p.Status.Phase = corev1.PodPending

	assert.False(t, isPodReady(&p))
}
