Use Cadence cli to interact with the worker. A few commands were needed to use the cadence cli: 

1. git clone https://github.com/uber/cadence.git - clone cadence
2. cd cadence - navigate to cadence folder
3. git submodule update --init - initialise submodules (cadence)
4. make cadence - compiles cadence binary

After these steps you can use the cadence cli to interact with workers/workflows, or literally anything else. e.g.:
cadence --domain test-domain-deimian domain describe
cadence --domain test-domain-deimian workflow start --et 60 --tl test-worker --workflow_type main.helloWorldWorkflow --input '"Deimian"'
cli tool | domain name | entity | operation | execution timeout 60s | task list name | workflow type name | input argument(s)       
