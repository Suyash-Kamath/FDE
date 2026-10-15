// Plain task logic (port of task_tools.py) - no MCP code in here.
// server.js wraps these functions as MCP tools.

const tasks = [];

export function createTaskValue(title) {
  tasks.push(title);
  return `Task created: ${title}`;
}

export function listTaskValues() {
  return [...tasks];
}

export function completeTaskValue(title) {
  const index = tasks.indexOf(title);
  if (index !== -1) {
    tasks.splice(index, 1);
    return `Task completed: ${title}`;
  }

  return `Task not found: ${title}`;
}
