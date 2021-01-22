# statefulPod Pod 扩容

## 测试

### 运行创建 Pod 脚本

1. 执行脚本
    - 若是脚本出现/bin/bash^M: bad interpreter: 没有那个文件或目录错误，请编辑脚本 set ff=unix 保存退出即可
    
    ```shell
    ./expansionPod
    ```
    
### 测试结果

1. 查看结果

    ```console
    #运行 kubectl get pod -n ldy --watch 可以看到 pod 被依次创建
    $ kubectl get pod -n ldy --watch
    #运行结果
    NAME                READY   STATUS    RESTARTS   AGE
    test-statefulpod0   0/1     Pending   0          0s
    test-statefulpod0   0/1     Pending   0          0s
    test-statefulpod0   0/1     ContainerCreating   0          0s
    test-statefulpod0   1/1     Running             0          2s
    test-statefulpod1   0/1     Pending             0          0s
    test-statefulpod1   0/1     Pending             0          0s
    test-statefulpod1   0/1     ContainerCreating   0          0s
    test-statefulpod1   1/1     Running             0          3s
    test-statefulpod2   0/1     Pending             0          0s
    test-statefulpod2   0/1     Pending             0          0s
    test-statefulpod2   0/1     ContainerCreating   0          0s
    test-statefulpod2   1/1     Running             0          2s
    test-statefulpod3   0/1     Pending             0          0s
    test-statefulpod3   0/1     Pending             0          0s
    test-statefulpod3   0/1     ContainerCreating   0          0s
    test-statefulpod3   1/1     Running             0          3s
    test-statefulpod4   0/1     Pending             0          0s
    test-statefulpod4   0/1     Pending             0          0s
    test-statefulpod4   0/1     ContainerCreating   0          1s
    test-statefulpod4   1/1     Running             0          3s
    test-statefulpod5   0/1     Pending             0          0s
    test-statefulpod5   0/1     Pending             0          0s
    test-statefulpod5   0/1     ContainerCreating   0          0s
    test-statefulpod5   1/1     Running             0          3s
    test-statefulpod6   0/1     Pending             0          0s
    test-statefulpod6   0/1     Pending             0          0s
    test-statefulpod6   0/1     ContainerCreating   0          0s
    test-statefulpod6   1/1     Running             0          2s
    test-statefulpod7   0/1     Pending             0          0s
    test-statefulpod7   0/1     Pending             0          0s
    test-statefulpod7   0/1     ContainerCreating   0          0s
    test-statefulpod7   1/1     Running             0          3s
    ```