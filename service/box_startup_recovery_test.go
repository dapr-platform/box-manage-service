package service

import (
	"box-manage-service/models"
	"testing"
)

// TestShouldRestoreTask 验证盒子重启时的任务恢复判定。
//
// 回归线上故障：恢复逻辑无条件重新下发全部任务，导致用户主动停止的
// 断流任务每次重启都被拉起，持续重连把 box-app 拖到死锁。
func TestShouldRestoreTask(t *testing.T) {
	cases := []struct {
		name string
		task *models.Task
		want bool
	}{
		{
			name: "开启自启且未取消 -> 恢复",
			task: &models.Task{AutoStart: true, Status: models.TaskStatusRunning},
			want: true,
		},
		{
			name: "未开启自启 -> 不恢复",
			task: &models.Task{AutoStart: false, Status: models.TaskStatusRunning},
			want: false,
		},
		{
			name: "已取消 -> 不恢复（即使开了自启）",
			task: &models.Task{AutoStart: true, Status: models.TaskStatusCancelled},
			want: false,
		},
		{
			name: "已取消且未自启 -> 不恢复",
			task: &models.Task{AutoStart: false, Status: models.TaskStatusCancelled},
			want: false,
		},
		{
			name: "nil 任务 -> 不恢复",
			task: nil,
			want: false,
		},
	}
	for _, c := range cases {
		if got := shouldRestoreTask(c.task); got != c.want {
			t.Fatalf("%s: shouldRestoreTask() = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestShouldRestoreTaskSkipsStoppedTask 单独锁定「用户停掉的任务重启后不被拉起」。
func TestShouldRestoreTaskSkipsStoppedTask(t *testing.T) {
	// 这是线上出问题的那个任务的典型状态：auto_start=0，用户已停止
	stoppedTask := &models.Task{
		TaskID:    "sync_9_042101",
		AutoStart: false,
		Status:    models.TaskStatusCancelled,
		RunStatus: models.RunStatusStopped,
	}
	if shouldRestoreTask(stoppedTask) {
		t.Fatal("用户停止的任务不应在盒子重启后被恢复")
	}
}
