# Deploying torch vision detect function

## HW requirements
1.  AMD EPYC 7302P 16-Core Processor - ~90ms
2. Intel(R) Xeon(R) CPU E5-2683 v3 - ~200ms

Recommendation: use 1.

## Setup:
1. Docker
Pull [asarma31/action-python-v3.11](https://hub.docker.com/r/asarma31/action-python-v3.11)

OR 

Build  & push to dockerhub using Dockerfile

2. Modify [create_action.sh](./create_action.sh) to deploy function
3. Use [invoke.py](./invoke.py) to test function
4. Remove action using [remove_action.sh](./remove_action.sh)

## Gotchas
- Standalone Openwhisk does not have sufficient default memory to instantiate container. Please add memory limts to `whisk` config as follows:
```yaml
  container-pool {
    user-memory: 4096 m
  }
  memory {
    min = 128 m
    max = 2048 m
    std = 256 m
  }

```
