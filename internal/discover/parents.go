package discover

import (
	"strings"

	"github.com/AymanZahran/ai9s/internal/model"
)

// noteParent records that childNative was launched by parentNative.
func noteParent(parents map[string]string, agent, childNative, parentNative string) {
	if parents == nil {
		return
	}
	childNative = strings.TrimSpace(childNative)
	parentNative = strings.TrimSpace(parentNative)
	if childNative == "" || parentNative == "" || childNative == parentNative {
		return
	}
	parents[model.ID(agent, childNative)] = model.ID(agent, parentNative)
}

func parentOf(agent, parentNative string) string {
	parentNative = strings.TrimSpace(parentNative)
	if parentNative == "" {
		return ""
	}
	return model.ID(agent, parentNative)
}
