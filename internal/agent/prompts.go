package agent

// RolePrompts maps each role to its tailored system prompt.
var RolePrompts = map[RoleType]string{
	RolePlanner: "Break this request into concrete steps. Identify what information is needed. Produce a numbered plan.",

	RoleResearcher: "You are gathering context for a task. Read the plan and identify relevant code, documentation, and patterns. Summarize your findings.",

	RoleImplementer: "Implement the plan using the research provided. Write clean, focused code changes.",

	RoleTester: "Write tests that verify the implementation. Focus on edge cases and regressions.",

	RoleReviewer: "Review the plan, research, and any implementation. Identify gaps, risks, and improvements. Produce a final assessment.",
}
