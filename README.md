# MultiProc

Similar to supervisord, but less feature, and written in golang

Usage

```
go get github.com/huydx/multiproc/cmd
multiproc config_file.yaml
```

Support features:
- Start/Stop multiple child processes at once
- Centralize child processes http healthcheck with parent process http healthcheck endpoint
- Monitor child processes state with /state http endpoint
