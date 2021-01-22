# statefulPod Pod 缩容

## 测试

### 运行创建 Pod 脚本

1. 执行脚本
    - 若是脚本出现/bin/bash^M: bad interpreter: 没有那个文件或目录错误，请编辑脚本 set ff=unix 保存退出即可
    
    ```shell
    ./shrinkPod.sh
    ```
    
### 测试结果

1. 查看结果

    ```console
    #运行 kubectl get pod -n ldy --watch 可以看到 pod 被依次删除
    $ kubectl get pod -n ldy --watch
    #运行结果
    NAME                READY   STATUS    RESTARTS   AGE
    NAME                READY   STATUS    RESTARTS   AGE
    test-statefulpod0   1/1     Running   0          5m38s
    test-statefulpod1   1/1     Running   0          5m36s
    test-statefulpod2   1/1     Running   0          5m33s
    test-statefulpod3   1/1     Running   0          5m31s
    test-statefulpod4   1/1     Running   0          5m28s
    test-statefulpod5   1/1     Running   0          5m25s
    test-statefulpod6   1/1     Running   0          5m22s
    test-statefulpod7   1/1     Running   0          5m20s
    test-statefulpod7   1/1     Terminating   0          6m56s
    test-statefulpod7   0/1     Terminating   0          6m57s
    test-statefulpod7   0/1     Terminating   0          7m6s
    test-statefulpod7   0/1     Terminating   0          7m6s
    test-statefulpod6   1/1     Terminating   0          7m12s
    test-statefulpod6   0/1     Terminating   0          7m14s
    test-statefulpod6   0/1     Terminating   0          7m15s
    test-statefulpod6   0/1     Terminating   0          7m15s
    test-statefulpod5   1/1     Terminating   0          7m20s
    test-statefulpod5   0/1     Terminating   0          7m22s
    test-statefulpod5   0/1     Terminating   0          7m32s
    test-statefulpod5   0/1     Terminating   0          7m32s
    test-statefulpod4   1/1     Terminating   0          7m39s
    test-statefulpod4   0/1     Terminating   0          7m41s
    test-statefulpod4   0/1     Terminating   0          7m45s
    test-statefulpod4   0/1     Terminating   0          7m45s
    ```