package mcpusage

const fallbackAgentTokensPerSecond = 75.0

var agentTokensPerSecondByTool = map[string]float64{}

func agentTPS(tool string) float64 {
	if v, ok := agentTokensPerSecondByTool[tool]; ok {
		return v
	}
	return fallbackAgentTokensPerSecond
}

func estAgentTimeMs(tool string, rawTokens int) int {
	return int(float64(rawTokens) / agentTPS(tool) * 1000)
}

func timeSavedMs(estAgent, actual int) int {
	if estAgent < actual {
		return 0
	}
	return estAgent - actual
}
