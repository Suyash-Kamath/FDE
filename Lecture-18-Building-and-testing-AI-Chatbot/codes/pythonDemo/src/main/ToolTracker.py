from contextvars import ContextVar
from contextlib import contextmanager


class ToolTracker:
    def __init__(self):
        self._state = ContextVar("tool_trace")

    @contextmanager
    def scope(self):
        token = self._state.set({"toolsUsed": [], "toolArguments": {}, "toolResults": {}})
        try:
            yield
        finally:
            self._state.reset(token)

    def clear(self):
        self._state.set({"toolsUsed": [], "toolArguments": {}, "toolResults": {}})

    def record(self, name, argument, result):
        state = self._state.get()
        if name not in state["toolsUsed"]:
            state["toolsUsed"].append(name)
        state["toolArguments"][name] = argument
        state["toolResults"][name] = result

    def getToolsUsed(self):
        return list(self._state.get()["toolsUsed"])

    def getToolArguments(self):
        return dict(self._state.get()["toolArguments"])

    def getToolResults(self):
        return dict(self._state.get()["toolResults"])
