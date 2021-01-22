# statefulPod Pod 维护

## 测试

### 运行创建 Pod 脚本

1. 手动删除 pod
    - 若是脚本出现/bin/bash^M: bad interpreter: 没有那个文件或目录错误，请编辑脚本 set ff=unix 保存退出即可
    - 运行脚本后边的数字是 Pod 的编号
    
    ```shell
    ./maintainPod.sh 2
    ```
    
2. 断开节点
    - 脚本后面的参数是 1 nodeName 断开; 2 nodeName 连接
    
        ```shell script
        ./nodeDisconnected.sh 1 nodeName (1表示断开 nodeName根据实际情况填写)
        ```
### 测试结果

1. 手动删除 Pod 结果

    ```console
    #运行 kubectl get pod -n ldy --watch 可以看到 pod 被依次删除
    $ kubectl get pod -n ldy --watch
    #运行结果
    NAME                READY   STATUS    RESTARTS   AGE
    test-statefulpod0   1/1     Running   0          14m
    test-statefulpod1   1/1     Running   0          14m
    test-statefulpod2   1/1     Running   0          14m
    test-statefulpod3   1/1     Running   0          14m
    test-statefulpod2   1/1     Terminating   0          17m
    test-statefulpod2   0/1     Terminating   0          17m
    test-statefulpod2   0/1     Terminating   0          17m
    test-statefulpod2   0/1     Terminating   0          17m
    test-statefulpod2   0/1     Pending       0          0s
    test-statefulpod2   0/1     Pending       0          0s
    test-statefulpod2   0/1     ContainerCreating   0          0s
    test-statefulpod2   1/1     Running             0          2s
    ```

2. 断开节点 结果

    ```console
   #断开前查看pod所在节点
   $ kubectl get pod -n ldy -o wide
   NAME                READY   STATUS    RESTARTS   AGE   IP              NODE                          NOMINATED NODE   READINESS GATES
   test-statefulpod0   1/1     Running   0          29s   10.217.5.102    p47664v.hulk.shbt.qihoo.net   <none>           <none>
   test-statefulpod1   1/1     Running   0          26s   10.217.13.193   p47656v.hulk.shbt.qihoo.net   <none>           <none>
   test-statefulpod2   1/1     Running   0          24s   10.217.5.201    p47664v.hulk.shbt.qihoo.net   <none>           <none>
   test-statefulpod3   1/1     Running   0          21s   10.217.13.93    p47656v.hulk.shbt.qihoo.net   <none>           <none>
    
   #运行脚本后(请等待2分钟左右)
   NAME                READY   STATUS    RESTARTS   AGE    IP              NODE                          NOMINATED NODE   READINESS GATES
   test-statefulpod0   1/1     Running   0          21s    10.217.13.245   p47656v.hulk.shbt.qihoo.net   <none>           <none>
   test-statefulpod1   1/1     Running   0          118s   10.217.13.193   p47656v.hulk.shbt.qihoo.net   <none>           <none>
   test-statefulpod2   1/1     Running   0          20s    10.217.13.124   p47656v.hulk.shbt.qihoo.net   <none>           <none>
   test-statefulpod3   1/1     Running   0          113s   10.217.13.93    p47656v.hulk.shbt.qihoo.net   <none>           <none>
    ```
 
