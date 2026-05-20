import type { RunTask } from "../../api/types";
import { taskRoleLabel, taskTone } from "../../lib/format";

export function RoleTimeline({ tasks }: { tasks: RunTask[] }) {
  return (
    <div className="role-flow" aria-label="Role flow">
      {tasks.map((task, index) => (
        <div
          className={"role-step " + taskTone(task.status)}
          key={task.task_id}
        >
          <span>{index + 1}</span>
          <strong>{taskRoleLabel(task)}</strong>
          <small>{task.status}</small>
        </div>
      ))}
    </div>
  );
}
