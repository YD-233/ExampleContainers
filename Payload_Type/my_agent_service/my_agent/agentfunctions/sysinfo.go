package agentfunctions

import (
	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
)

func init() {
	agentstructs.AllPayloadData.Get("my_agent").AddCommand(agentstructs.Command{
		Name:                "sysinfo",
		HelpString:          "sysinfo",
		Description:         "收集当前主机基础系统信息",
		Version:             1,
		Author:              "@zhujiayi",
		MitreAttackMappings: []string{"T1082"},
		TaskFunctionCreateTasking: func(taskData *agentstructs.PTTaskMessageAllData) agentstructs.PTTaskCreateTaskingMessageResponse {
			return agentstructs.PTTaskCreateTaskingMessageResponse{
				Success: true,
				TaskID:  taskData.Task.ID,
			}
		},
	})
}
