# This is an excercise in writing an app completely with LLMs, no hand editing of code. Let's call it homelab-dashboard, as it'll interface with my homelab cluster and make a dashboard out of it

### Design
You have a rough sketch of a design i want in the design folder, especially the DESIGN.md file. There are also html files of the design i want.

### Technologies
I expect the app to be written in golang, and compile into a docker container, that accepts any and all needed options as environment variables. I will be deploying it in the kubernetes cluster of course, but make it an option to run it in a local docker container. All needed docker stuff should be in a .docker directory. The container will run rootless and with an RO filesystem

### Helpers
You should be able to access my cluster through tools, I have kubernetes-mcp and flux-operator-mcp running, along some others. Use Context7 when unsure about how things should be, you should have up to date documentation there. Feel free to use subagents as well
