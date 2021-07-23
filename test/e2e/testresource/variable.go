package testresource

import "fmt"

const (
	StatefulPodGroup    = "iapetos.foundary-cloud.io"
	StatefulPodVersion  = "v1"
	StatefulPodKind     = "StatefulPod"
	StatefulPodResource = "statefulpods"
	Image               = "uhub.service.ucloud.cn/infra/nginx:1.17.9"
	BasicName           = "stateful-pod-test"
	BasicNameSpace      = "ldy"
	BasicSize           = 3
	Basic               = 1
	WithService         = 2
	WithPVC             = 3
	WithStaticPvc       = 4
	PvName0             = "ldy.pvc-a452ed33-8a36-4fd5-bbb6-09118d1235f3"
	PvName1             = "ldy.pvc-7c0e7571-08c6-4f05-84ce-8098738f4f5c"
	PvName2             = "ldy.pvc-c005d9d0-5dbf-4a7c-9b31-beb6bf5a704e"
)

var BasicTemplate = fmt.Sprintf(`
apiVersion: %s/%s
kind: %s
metadata:
  name: %s
  namespace: %s
spec:
  selector:
    matchLabels:
      father: %s
  size: %d
  podTemplate:
    containers:
    - name: %s
      image: %s
`, StatefulPodGroup, StatefulPodVersion, StatefulPodKind, BasicName, BasicNameSpace, BasicName, BasicSize, "test", Image)

var WithServiceTemplate = fmt.Sprintf(`
apiVersion: %s/%s
kind: %s
metadata:
  name: %s
  namespace: %s
spec:
  selector:
    matchLabels:
      father: %s
  size: %d
  podTemplate:
    containers:
    - name: %s
      image: %s
  serviceTemplate:
    selector:
      app: test-stateful-pod
      test: "true"
    clusterIP: None
    ports:
    - port: 80
      targetPort: 80
`, StatefulPodGroup, StatefulPodVersion, StatefulPodKind, BasicName, BasicNameSpace, BasicName, BasicSize, "test", Image)

var WithPvcTemplate = fmt.Sprintf(`
apiVersion: %s/%s
kind: %s
metadata:
  name: %s
  namespace: %s
spec:
  pvRecyclePolicy: "Delete"
  selector:
    matchLabels:
      father: %s
  size: %d
  podTemplate:
    containers:
    - name: %s
      image: %s
      volumeMounts:
      - name: test
        mountPath: /test
      - name: host-type
        mountPath: /data
    volumes:
    - name: test
      persistentVolumeClaim:
        claimName: data
    - name: host-type
      hostPath:
        path: /data
        type: DirectoryOrCreate
  pvcTemplate:
    storageClassName: "kubevirt-hostpath-provisioner"
    accessModes:
      - ReadWriteOnce
    resources:
      requests:
        storage: 1Gi
  serviceTemplate:
    selector:
      app: test-stateful-pod
      test: "true"
    clusterIP: None
    ports:
    - port: 80
      targetPort: 80
`, StatefulPodGroup, StatefulPodVersion, StatefulPodKind, BasicName, BasicNameSpace, BasicName, BasicSize, "test", Image)

var WithStaticPvcTemplate = fmt.Sprintf(`
apiVersion: %s/%s
kind: %s
metadata:
  name: %s
  namespace: %s
spec:
  pvRecyclePolicy: "Retain"
  selector:
    matchLabels:
      father: %s
  size: %d
  podTemplate:
    containers:
    - name: %s
      image: %s
      volumeMounts:
      - name: test
        mountPath: /test
      - name: host-type
        mountPath: /data
    volumes:
    - name: test
      persistentVolumeClaim:
        claimName: data
    - name: host-type
      hostPath:
        path: /data
        type: DirectoryOrCreate
  pvcTemplate:
    storageClassName: "kubevirt-hostpath-provisioner"
    accessModes:
      - ReadWriteOnce
    resources:
      requests:
        storage: 1Gi
  serviceTemplate:
    selector:
      app: test-stateful-pod
      test: "true"
    clusterIP: None
    ports:
    - port: 80
      targetPort: 80
  pvNames:
  - %s
  - %s
  - %s
`, StatefulPodGroup, StatefulPodVersion, StatefulPodKind, BasicName, BasicNameSpace, BasicName, BasicSize, "test", Image, PvName0, PvName1, PvName2)
