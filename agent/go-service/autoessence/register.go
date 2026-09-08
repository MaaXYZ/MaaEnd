package autoessence

import maa "github.com/MaaXYZ/maa-framework-go/v4"

// Register registers AutoEssence inventory custom components.
func Register() {
	maa.AgentServerRegisterCustomAction("AutoEssenceLoadInventoryAction", &LoadInventoryAction{})
}
