package estimate

// Tokens estimates the token count for a tool output.
// Input: raw bytes of the tool_result field, plus the tool name and weight map.
func Tokens(tool string, outputBytes int, weights map[string]float64) int {
	multiplier, ok := weights[tool]
	if !ok {
		if def, ok := weights["default"]; ok {
			multiplier = def
		} else {
			multiplier = 1.0
		}
	}
	return int(float64(outputBytes) / 4.0 * multiplier)
}
