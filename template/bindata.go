package template

import (
	"fmt"
	"strings"
)
var _bindata_go = []byte(``)

func bindata_go() ([]byte, error) {
	return _bindata_go, nil
}

var _che_1_0_58_openshift_yaml = []byte(`---
apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: ServiceAccount
  metadata:
    labels:
      provider: fabric8
      project: che
      version: 1.0.54
      group: io.fabric8.online.apps
    name: che
- apiVersion: v1
  kind: Service
  metadata:
    annotations:
      fabric8.io/app-menu: development
      fabric8.io/git-commit: 36ca22dc526863e8a154fc04b405e93d13a1cc58
      fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-online.git/apps/che
      fabric8.io/scm-url: http://github.com/fabric8io/fabric8-online/apps/che
      fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-online/master/apps/che/src/main/fabric8/icon.png
      fabric8.io/git-branch: release-v1.0.54
      fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-online.git/apps/che
      fabric8.io/scm-tag: fabric8-online-2.0.1
    labels:
      project: che
      provider: fabric8
      version: 1.0.54
      group: io.fabric8.online.apps
    name: che-host
  spec:
    ports:
    - name: http
      port: 8080
      protocol: TCP
      targetPort: 8080
    selector:
      project: che
      provider: fabric8
      group: io.fabric8.online.apps
- apiVersion: v1
  kind: RoleBinding
  metadata:
    labels:
      provider: fabric8
      project: che
      version: 1.0.54
      group: io.fabric8.online.apps
    name: che
  roleRef:
    name: admin
  subjects:
  - kind: ServiceAccount
    name: che
- apiVersion: v1
  kind: PersistentVolumeClaim
  metadata:
    labels:
      provider: fabric8
      project: che
      version: 1.0.54
      group: io.fabric8.online.apps
    name: che-conf-volume
  spec:
    accessModes:
    - ReadWriteOnce
    resources:
      requests:
        storage: 1Gi
- apiVersion: v1
  kind: PersistentVolumeClaim
  metadata:
    labels:
      provider: fabric8
      project: che
      version: 1.0.54
      group: io.fabric8.online.apps
    name: che-data-volume
  spec:
    accessModes:
    - ReadWriteOnce
    resources:
      requests:
        storage: 1Gi
- apiVersion: v1
  kind: PersistentVolumeClaim
  metadata:
    labels:
      provider: fabric8
      project: che
      version: 1.0.54
      group: io.fabric8.online.apps
    name: claim-che-workspace
  spec:
    accessModes:
    - ReadWriteOnce
    resources:
      requests:
        storage: 1Gi
- apiVersion: v1
  kind: ConfigMap
  metadata:
    labels:
      provider: fabric8
      project: che
      version: 1.0.54
      group: io.fabric8.online.apps
    name: che
  data:
    hostname-http: aslak-dsaas-che.tsrv.devshift.net
    workspace-storage: /home/user/che/workspaces
    workspace-storage-create-folders: "false"
    local-conf-dir: /etc/conf
    openshift-serviceaccountname: che
    che-server-evaluation-strategy: single-port
    log-level: INFO
    docker-connector: openshift
    port: "8080"
    remote-debugging-enabled: "false"
- apiVersion: v1
  kind: DeploymentConfig
  metadata:
    annotations:
      fabric8.io/git-commit: 36ca22dc526863e8a154fc04b405e93d13a1cc58
      fabric8.io/metrics-path: dashboard/file/kubernetes-pods.json/?var-project=che&var-version=1.0.54
      fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-online.git/apps/che
      fabric8.io/scm-url: http://github.com/fabric8io/fabric8-online/apps/che
      fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-online/master/apps/che/src/main/fabric8/icon.png
      fabric8.io/git-branch: release-v1.0.54
      fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-online.git/apps/che
      fabric8.io/scm-tag: fabric8-online-2.0.1
    labels:
      provider: fabric8
      project: che
      version: 1.0.54
      group: io.fabric8.online.apps
    name: che
  spec:
    replicas: 1
    selector:
      project: che
      provider: fabric8
      group: io.fabric8.online.apps
    template:
      metadata:
        annotations:
          fabric8.io/git-commit: 36ca22dc526863e8a154fc04b405e93d13a1cc58
          fabric8.io/metrics-path: dashboard/file/kubernetes-pods.json/?var-project=che&var-version=1.0.54
          fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-online.git/apps/che
          fabric8.io/scm-url: http://github.com/fabric8io/fabric8-online/apps/che
          fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-online/master/apps/che/src/main/fabric8/icon.png
          fabric8.io/git-branch: release-v1.0.54
          fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-online.git/apps/che
          fabric8.io/scm-tag: fabric8-online-2.0.1
        labels:
          provider: fabric8
          project: che
          version: 1.0.54
          group: io.fabric8.online.apps
      spec:
        containers:
        - env:
          - name: CHE_DOCKER_IP_EXTERNAL
            valueFrom:
              configMapKeyRef:
                key: hostname-http
                name: che
          - name: CHE_WORKSPACE_STORAGE
            valueFrom:
              configMapKeyRef:
                key: workspace-storage
                name: che
          - name: CHE_WORKSPACE_STORAGE_CREATE_FOLDERS
            valueFrom:
              configMapKeyRef:
                key: workspace-storage-create-folders
                name: che
          - name: CHE_LOCAL_CONF_DIR
            valueFrom:
              configMapKeyRef:
                key: local-conf-dir
                name: che
          - name: CHE_OPENSHIFT_PROJECT
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
          - name: CHE_OPENSHIFT_SERVICEACCOUNTNAME
            valueFrom:
              configMapKeyRef:
                key: openshift-serviceaccountname
                name: che
          - name: CHE_DOCKER_SERVER__EVALUATION__STRATEGY
            valueFrom:
              configMapKeyRef:
                key: che-server-evaluation-strategy
                name: che
          - name: CHE_LOG_LEVEL
            valueFrom:
              configMapKeyRef:
                key: log-level
                name: che
          - name: CHE_PORT
            valueFrom:
              configMapKeyRef:
                key: port
                name: che
          - name: CHE_DOCKER_CONNECTOR
            valueFrom:
              configMapKeyRef:
                key: docker-connector
                name: che
          - name: CHE_DEBUG_SERVER
            valueFrom:
              configMapKeyRef:
                key: remote-debugging-enabled
                name: che
          image: mariolet/che-server:nightly
          imagePullPolicy: Always
          livenessProbe:
            initialDelaySeconds: 120
            tcpSocket:
              port: 8080
            timeoutSeconds: 10
          name: che
          ports:
          - containerPort: 8080
            name: http
          - containerPort: 8000
            name: http-debug
          readinessProbe:
            initialDelaySeconds: 20
            tcpSocket:
              port: 8080
            timeoutSeconds: 10
          volumeMounts:
          - mountPath: /conf
            name: che-conf-volume
            readOnly: false
          - mountPath: /data
            name: che-data-volume
        serviceAccountName: che
        volumes:
        - name: che-conf-volume
          persistentVolumeClaim:
            claimName: che-conf-volume
        - name: che-data-volume
          persistentVolumeClaim:
            claimName: che-data-volume
    triggers:
    - type: ConfigChange
- apiVersion: v1
  kind: Route
  metadata:
    labels:
      provider: fabric8
      project: che
      version: 1.0.54
      group: io.fabric8.online.apps
    name: che
  spec:
    to:
      kind: Service
      name: che-host`)

func che_1_0_58_openshift_yaml() ([]byte, error) {
	return _che_1_0_58_openshift_yaml, nil
}

var _content_repository_2_2_330_openshift_yaml = []byte(`---
apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: Service
  metadata:
    annotations:
      fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-devops/master/content-repository/src/main/fabric8/icon.svg
      prometheus.io/port: "9180"
      prometheus.io/scrape: "true"
      fabric8.io/git-commit: 5c453edb04f1c8d5fc34228a671300d8523b0274
      fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-devops.git/content-repository
      fabric8.io/scm-url: http://github.com/fabric8io/fabric8-devops/content-repository
      fabric8.io/git-branch: release-v2.2.330
      fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-devops.git/content-repository
      fabric8.io/scm-tag: fabric8-devops-2.0.1
    labels:
      provider: fabric8
      project: content-repository
      version: 2.2.330
      group: io.fabric8.devops.apps
      expose: "true"
    name: content-repository
  spec:
    ports:
    - port: 80
      protocol: TCP
      targetPort: 8080
    selector:
      project: content-repository
      provider: fabric8
      group: io.fabric8.devops.apps
- apiVersion: v1
  kind: PersistentVolumeClaim
  metadata:
    annotations:
    labels:
      provider: fabric8
      project: content-repository
      version: 2.2.330
      group: io.fabric8.devops.apps
    name: content-repository
  spec:
    accessModes:
    - ReadWriteOnce
    resources:
      requests:
        storage: 1Gi
- apiVersion: v1
  kind: DeploymentConfig
  metadata:
    annotations:
      fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-devops/master/content-repository/src/main/fabric8/icon.svg
      fabric8.io/git-commit: 5c453edb04f1c8d5fc34228a671300d8523b0274
      fabric8.io/metrics-path: dashboard/file/kubernetes-pods.json/?var-project=content-repository&var-version=2.2.330
      fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-devops.git/content-repository
      fabric8.io/scm-url: http://github.com/fabric8io/fabric8-devops/content-repository
      fabric8.io/git-branch: release-v2.2.330
      fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-devops.git/content-repository
      fabric8.io/scm-tag: fabric8-devops-2.0.1
    labels:
      provider: fabric8
      project: content-repository
      version: 2.2.330
      group: io.fabric8.devops.apps
    name: content-repository
  spec:
    replicas: 1
    selector:
      project: content-repository
      provider: fabric8
      version: 2.2.330
      group: io.fabric8.devops.apps
    template:
      metadata:
        annotations:
          fabric8.io/git-commit: 5c453edb04f1c8d5fc34228a671300d8523b0274
          fabric8.io/metrics-path: dashboard/file/kubernetes-pods.json/?var-project=content-repository&var-version=2.2.330
          fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-devops.git/content-repository
          fabric8.io/scm-url: http://github.com/fabric8io/fabric8-devops/content-repository
          fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-devops/master/content-repository/src/main/fabric8/icon.svg
          fabric8.io/git-branch: release-v2.2.330
          fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-devops.git/content-repository
          fabric8.io/scm-tag: fabric8-devops-2.0.1
        labels:
          provider: fabric8
          project: content-repository
          version: 2.2.330
          group: io.fabric8.devops.apps
      spec:
        containers:
        - env:
          - name: KUBERNETES_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
          image: fabric8/alpine-caddy:2.2.330
          imagePullPolicy: IfNotPresent
          name: content-repository
          ports:
          - containerPort: 8080
            name: http
          - containerPort: 9180
            name: prometheus
          resources:
            limits:
              cpu: "0"
              memory: "0"
            requests:
              cpu: "0"
              memory: "0"
          volumeMounts:
          - mountPath: /var/www/html
            name: content
            readOnly: false
        volumes:
        - name: content
          persistentVolumeClaim:
            claimName: content-repository
    triggers:
    - type: ConfigChange
- apiVersion: v1
  kind: Route
  metadata:
    labels:
      provider: fabric8
      project: content-repository
      version: 2.2.330
      group: io.fabric8.devops.apps
    name: content-repository
  spec:
    tls:
      insecureEdgeTerminationPolicy: Redirect
      termination: edge
    to:
      kind: Service
      name: content-repository
`)

func content_repository_2_2_330_openshift_yaml() ([]byte, error) {
	return _content_repository_2_2_330_openshift_yaml, nil
}

var _fabric8_online_yaml = []byte(`---
apiVersion: v1
kind: Template
metadata:
  labels:
    provider: fabric8
    project: fabric8-online-team-environments
    version: 1.0.58
    group: io.fabric8.online.packages
  name: fabric8-online-team-envi
objects:
- apiVersion: v1
  kind: Project
  metadata:
    annotations:
      openshift.io/description: ${PROJECT_DESCRIPTION}
      openshift.io/display-name: ${PROJECT_DISPLAYNAME}
      openshift.io/requester: ${PROJECT_REQUESTING_USER}
    labels:
      provider: fabric8
      project: fabric8-online-team-environments
      version: 1.0.58
      group: io.fabric8.online.packages
    name: ${PROJECT_NAME}
- apiVersion: v1
  kind: RoleBinding
  metadata:
    name: admin
    namespace: ${PROJECT_NAME}
  subjects:
    - kind: User
      name: ${PROJECT_ADMIN_USER}
  roleRef:
    name: admin
- apiVersion: v1
  kind: RoleBinding
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-team-environments
      version: 1.0.58
      group: io.fabric8.online.packages
    name: system:deployers
    namespace: ${PROJECT_NAME}
  roleRef:
    name: system:deployer
  subjects:
  - kind: ServiceAccount
    namespace: ${PROJECT_NAME}
    name: deployer
- apiVersion: v1
  kind: RoleBinding
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-team-environments
      version: 1.0.58
      group: io.fabric8.online.packages
    name: system:image-builders
    namespace: ${PROJECT_NAME}
  roleRef:
    name: system:image-builder
  subjects:
  - kind: ServiceAccount
    namespace: ${PROJECT_NAME}
    name: builder
- apiVersion: v1
  kind: RoleBinding
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-team-environments
      version: 1.0.58
      group: io.fabric8.online.packages
    name: system:image-pullers
    namespace: ${PROJECT_NAME}
  roleRef:
    name: system:image-puller
  subjects:
  - kind: SystemGroup
    name: system:serviceaccounts:${PROJECT_NAME}
parameters:
- name: PROJECT_NAME
- name: PROJECT_DISPLAYNAME
- name: PROJECT_DESCRIPTION
- name: PROJECT_ADMIN_USER
- name: PROJECT_REQUESTING_USER
`)

func fabric8_online_yaml() ([]byte, error) {
	return _fabric8_online_yaml, nil
}

var _jenkins_openshift_2_2_330_openshift_yaml = []byte(`---
apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: Secret
  metadata:
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 2.2.330
      group: io.fabric8.devops.apps
    name: jenkins-docker-cfg
  data:
    config.json: ""
  type: fabric8.io/jenkins-docker-cfg
- apiVersion: v1
  kind: Secret
  metadata:
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 2.2.330
      group: io.fabric8.devops.apps
    name: jenkins-git-ssh
  data:
    ssh-key: ""
    ssh-key.pub: ""
  type: fabric8.io/jenkins-git-ssh
- apiVersion: v1
  kind: Secret
  metadata:
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 2.2.330
      group: io.fabric8.devops.apps
    name: jenkins-hub-api-token
  data:
    hub: ""
  type: fabric8.io/jenkins-hub-api-token
- apiVersion: v1
  kind: Secret
  metadata:
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 2.2.330
      group: io.fabric8.devops.apps
    name: jenkins-maven-settings
  data:
    settings.xml: PHNldHRpbmdzPg0KICA8IS0tIHNldHMgdGhlIGxvY2FsIG1hdmVuIHJlcG9zaXRvcnkgb3V0c2lkZSBvZiB0aGUgfi8ubTIgZm9sZGVyIGZvciBlYXNpZXIgbW91bnRpbmcgb2Ygc2VjcmV0cyBhbmQgcmVwbyAtLT4NCiAgPGxvY2FsUmVwb3NpdG9yeT4ke3VzZXIuaG9tZX0vLm12bnJlcG9zaXRvcnk8L2xvY2FsUmVwb3NpdG9yeT4NCiAgPG1pcnJvcnM+DQogICAgPG1pcnJvcj4NCiAgICAgIDxpZD5uZXh1czwvaWQ+DQogICAgICA8bWlycm9yT2Y+ZXh0ZXJuYWw6KjwvbWlycm9yT2Y+DQogICAgICA8dXJsPmh0dHA6Ly9jZW50cmFsLm1hdmVuLm9yZy9tYXZlbjIvPC91cmw+DQogICAgPC9taXJyb3I+DQogIDwvbWlycm9ycz4NCg0KICA8IS0tIGxldHMgZGlzYWJsZSB0aGUgZG93bmxvYWQgcHJvZ3Jlc3MgaW5kaWNhdG9yIHRoYXQgZmlsbHMgdXAgbG9ncyAtLT4NCiAgPGludGVyYWN0aXZlTW9kZT5mYWxzZTwvaW50ZXJhY3RpdmVNb2RlPg0KDQogIDxzZXJ2ZXJzPg0KICAgIDxzZXJ2ZXI+DQogICAgICA8aWQ+bG9jYWwtbmV4dXM8L2lkPg0KICAgICAgPHVzZXJuYW1lPmFkbWluPC91c2VybmFtZT4NCiAgICAgIDxwYXNzd29yZD5hZG1pbjEyMzwvcGFzc3dvcmQ+DQogICAgPC9zZXJ2ZXI+DQogICAgPHNlcnZlcj4NCiAgICAgIDxpZD5uZXh1czwvaWQ+DQogICAgICA8dXNlcm5hbWU+YWRtaW48L3VzZXJuYW1lPg0KICAgICAgPHBhc3N3b3JkPmFkbWluMTIzPC9wYXNzd29yZD4NCiAgICA8L3NlcnZlcj4NCiAgICA8c2VydmVyPg0KICAgICAgPGlkPm9zcy1zb25hdHlwZS1zdGFnaW5nPC9pZD4NCiAgICAgIDx1c2VybmFtZT48L3VzZXJuYW1lPg0KICAgICAgPHBhc3N3b3JkPjwvcGFzc3dvcmQ+DQogICAgPC9zZXJ2ZXI+DQogIDwvc2VydmVycz4NCg0KICA8cHJvZmlsZXM+DQogICAgPHByb2ZpbGU+DQogICAgICA8aWQ+bmV4dXM8L2lkPg0KICAgICAgPHByb3BlcnRpZXM+DQogICAgICAgIDxhbHREZXBsb3ltZW50UmVwb3NpdG9yeT5sb2NhbC1uZXh1czo6ZGVmYXVsdDo6aHR0cDovL2NvbnRlbnQtcmVwb3NpdG9yeS9jb250ZW50L3JlcG9zaXRvcmllcy9zdGFnaW5nLzwvYWx0RGVwbG95bWVudFJlcG9zaXRvcnk+DQogICAgICAgIDxhbHRSZWxlYXNlRGVwbG95bWVudFJlcG9zaXRvcnk+bG9jYWwtbmV4dXM6OmRlZmF1bHQ6Omh0dHA6Ly9jb250ZW50LXJlcG9zaXRvcnkvY29udGVudC9yZXBvc2l0b3JpZXMvc3RhZ2luZy88L2FsdFJlbGVhc2VEZXBsb3ltZW50UmVwb3NpdG9yeT4NCiAgICAgICAgPGFsdFNuYXBzaG90RGVwbG95bWVudFJlcG9zaXRvcnk+bG9jYWwtbmV4dXM6OmRlZmF1bHQ6Omh0dHA6Ly9jb250ZW50LXJlcG9zaXRvcnkvY29udGVudC9yZXBvc2l0b3JpZXMvc25hcHNob3RzLzwvYWx0U25hcHNob3REZXBsb3ltZW50UmVwb3NpdG9yeT4NCiAgICAgIDwvcHJvcGVydGllcz4NCiAgICAgIDxyZXBvc2l0b3JpZXM+DQogICAgICAgIDxyZXBvc2l0b3J5Pg0KICAgICAgICAgIDxpZD5jZW50cmFsPC9pZD4NCiAgICAgICAgICA8dXJsPmh0dHA6Ly9jZW50cmFsPC91cmw+DQogICAgICAgICAgPHJlbGVhc2VzPjxlbmFibGVkPnRydWU8L2VuYWJsZWQ+PC9yZWxlYXNlcz4NCiAgICAgICAgICA8c25hcHNob3RzPjxlbmFibGVkPnRydWU8L2VuYWJsZWQ+PC9zbmFwc2hvdHM+DQogICAgICAgIDwvcmVwb3NpdG9yeT4NCiAgICAgIDwvcmVwb3NpdG9yaWVzPg0KICAgICAgPHBsdWdpblJlcG9zaXRvcmllcz4NCiAgICAgICAgPHBsdWdpblJlcG9zaXRvcnk+DQogICAgICAgICAgPGlkPmNlbnRyYWw8L2lkPg0KICAgICAgICAgIDx1cmw+aHR0cDovL2NlbnRyYWw8L3VybD4NCiAgICAgICAgICA8cmVsZWFzZXM+PGVuYWJsZWQ+dHJ1ZTwvZW5hYmxlZD48L3JlbGVhc2VzPg0KICAgICAgICAgIDxzbmFwc2hvdHM+PGVuYWJsZWQ+dHJ1ZTwvZW5hYmxlZD48L3NuYXBzaG90cz4NCiAgICAgICAgPC9wbHVnaW5SZXBvc2l0b3J5Pg0KICAgICAgPC9wbHVnaW5SZXBvc2l0b3JpZXM+DQogICAgPC9wcm9maWxlPg0KICAgIDxwcm9maWxlPg0KICAgICAgPGlkPnJlbGVhc2U8L2lkPg0KICAgICAgPHByb3BlcnRpZXM+DQogICAgICAgIDxncGcuZXhlY3V0YWJsZT5ncGc8L2dwZy5leGVjdXRhYmxlPg0KICAgICAgICA8Z3BnLnBhc3NwaHJhc2U+bXlzZWNyZXRwYXNzcGhyYXNlPC9ncGcucGFzc3BocmFzZT4NCiAgICAgIDwvcHJvcGVydGllcz4NCiAgICA8L3Byb2ZpbGU+DQogIDwvcHJvZmlsZXM+DQogIDxhY3RpdmVQcm9maWxlcz4NCiAgICA8IS0tbWFrZSB0aGUgcHJvZmlsZSBhY3RpdmUgYWxsIHRoZSB0aW1lIC0tPg0KICAgIDxhY3RpdmVQcm9maWxlPm5leHVzPC9hY3RpdmVQcm9maWxlPg0KICA8L2FjdGl2ZVByb2ZpbGVzPg0KPC9zZXR0aW5ncz4NCg==
  type: fabric8.io/secret-maven-settings
- apiVersion: v1
  kind: Secret
  metadata:
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 2.2.330
      group: io.fabric8.devops.apps
    name: jenkins-release-gpg
  data:
    trustdb.gpg: ""
    pubring.gpg: ""
    gpg.conf: ""
    secring.gpg: ""
  type: fabric8.io/jenkins-release-gpg
- apiVersion: v1
  kind: Secret
  metadata:
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 2.2.330
      group: io.fabric8.devops.apps
    name: jenkins-ssh-config
  data:
    config: ""
  type: fabric8.io/jenkins-ssh-config
- apiVersion: v1
  kind: ServiceAccount
  metadata:
    annotations:
      serviceaccounts.openshift.io/oauth-redirectreference.jenkins: '{"kind":"OAuthRedirectReference","apiVersion":"v1","reference":{"kind":"Route","name":"jenkins"}}'
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 2.2.330
      group: io.fabric8.devops.apps
    name: jenkins
- apiVersion: v1
  kind: Service
  metadata:
    annotations:
      fabric8.io/app-menu: development
      fabric8.io/git-commit: 5c453edb04f1c8d5fc34228a671300d8523b0274
      fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-devops.git/jenkins-openshift
      fabric8.io/scm-url: http://github.com/fabric8io/fabric8-devops/jenkins-openshift
      fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-devops/master/jenkins-openshift/src/main/fabric8/icon.svg
      fabric8.io/git-branch: release-v2.2.330
      fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-devops.git/jenkins-openshift
      fabric8.io/scm-tag: fabric8-devops-2.0.1
    labels:
      project: jenkins
      provider: fabric8
      expose: "false"
      version: 2.2.330
      group: io.fabric8.devops.apps
    name: jenkins
  spec:
    ports:
    - name: http
      port: 80
      protocol: TCP
      targetPort: 8080
    selector:
      project: jenkins-openshift
      provider: fabric8
      group: io.fabric8.devops.apps
- apiVersion: v1
  kind: Service
  metadata:
    annotations:
      fabric8.io/git-commit: 5c453edb04f1c8d5fc34228a671300d8523b0274
      fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-devops.git/jenkins-openshift
      fabric8.io/scm-url: http://github.com/fabric8io/fabric8-devops/jenkins-openshift
      fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-devops/master/jenkins-openshift/src/main/fabric8/icon.svg
      fabric8.io/git-branch: release-v2.2.330
      fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-devops.git/jenkins-openshift
      fabric8.io/scm-tag: fabric8-devops-2.0.1
    labels:
      project: jenkins
      provider: fabric8
      expose: "false"
      version: 2.2.330
      group: io.fabric8.devops.apps
    name: jenkins-jnlp
  spec:
    ports:
    - name: agent
      port: 50000
      protocol: TCP
      targetPort: 50000
    selector:
      project: jenkins-openshift
      provider: fabric8
      group: io.fabric8.devops.apps
- apiVersion: v1
  kind: RoleBinding
  metadata:
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 2.2.330
      group: io.fabric8.devops.apps
    name: edit-jenkins
  roleRef:
    name: edit
  subjects:
  - kind: ServiceAccount
    name: jenkins
- apiVersion: v1
  kind: PersistentVolumeClaim
  metadata:
    annotations:
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 2.2.330
      group: io.fabric8.devops.apps
    name: jenkins-home
  spec:
    accessModes:
    - ReadWriteOnce
    resources:
      requests:
        storage: 1Gi
- apiVersion: v1
  kind: PersistentVolumeClaim
  metadata:
    annotations:
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 2.2.330
      group: io.fabric8.devops.apps
    name: jenkins-mvn-local-repo
  spec:
    accessModes:
    - ReadWriteMany
    resources:
      requests:
        storage: 1Gi
- apiVersion: v1
  kind: DeploymentConfig
  metadata:
    annotations:
      fabric8.io/git-commit: 5c453edb04f1c8d5fc34228a671300d8523b0274
      fabric8.io/metrics-path: dashboard/file/kubernetes-pods.json/?var-project=jenkins-openshift&var-version=2.2.330
      fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-devops.git/jenkins-openshift
      fabric8.io/scm-url: http://github.com/fabric8io/fabric8-devops/jenkins-openshift
      fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-devops/master/jenkins-openshift/src/main/fabric8/icon.svg
      fabric8.io/git-branch: release-v2.2.330
      fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-devops.git/jenkins-openshift
      fabric8.io/scm-tag: fabric8-devops-2.0.1
    labels:
      fabric8.io/type: preview
      provider: fabric8
      project: jenkins-openshift
      version: 2.2.330
      group: io.fabric8.devops.apps
    name: jenkins
  spec:
    replicas: 1
    selector:
      project: jenkins-openshift
      provider: fabric8
      version: 2.2.330
      group: io.fabric8.devops.apps
    template:
      metadata:
        annotations:
          fabric8.io/git-commit: 5c453edb04f1c8d5fc34228a671300d8523b0274
          fabric8.io/metrics-path: dashboard/file/kubernetes-pods.json/?var-project=jenkins-openshift&var-version=2.2.330
          fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-devops.git/jenkins-openshift
          fabric8.io/scm-url: http://github.com/fabric8io/fabric8-devops/jenkins-openshift
          fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-devops/master/jenkins-openshift/src/main/fabric8/icon.svg
          fabric8.io/git-branch: release-v2.2.330
          fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-devops.git/jenkins-openshift
          fabric8.io/scm-tag: fabric8-devops-2.0.1
        labels:
          provider: fabric8
          project: jenkins-openshift
          version: 2.2.330
          group: io.fabric8.devops.apps
      spec:
        containers:
        - env:
          - name: GIT_COMMITTER_EMAIL
            value: fabric8@googlegroups.com
          - name: GIT_COMMITTER_NAME
            value: fabric8
          - name: OPENSHIFT_ENABLE_OAUTH
            value: "true"
          - name: OPENSHIFT_ENABLE_REDIRECT_PROMPT
            value: "true"
          - name: KUBERNETES_TRUST_CERTIFICATES
            value: "true"
          - name: KUBERNETES_MASTER
            value: https://kubernetes.default:443
          image: fabric8/jenkins-openshift:va28ca84
          imagePullPolicy: Always
          livenessProbe:
            failureThreshold: 30
            httpGet:
              path: /login
              port: 8080
            initialDelaySeconds: 420
            timeoutSeconds: 3
          name: jenkins
          ports:
          - containerPort: 50000
            name: slave
          - containerPort: 8080
            name: http
          readinessProbe:
            httpGet:
              path: /login
              port: 8080
            initialDelaySeconds: 3
            timeoutSeconds: 3
          resources:
            limits:
              memory: 350Mi
          volumeMounts:
          - mountPath: /var/lib/jenkins
            name: jenkins-home
            readOnly: false
        serviceAccountName: jenkins
        volumes:
        - name: jenkins-home
          persistentVolumeClaim:
            claimName: jenkins-home
    triggers:
    - type: ConfigChange
- apiVersion: v1
  kind: Route
  metadata:
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 2.2.330
      group: io.fabric8.devops.apps
    name: jenkins
  spec:
    tls:
      insecureEdgeTerminationPolicy: Redirect
      termination: edge
    to:
      kind: Service
      name: jenkins
`)

func jenkins_openshift_2_2_330_openshift_yaml() ([]byte, error) {
	return _jenkins_openshift_2_2_330_openshift_yaml, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		return f()
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() ([]byte, error){
	"bindata.go": bindata_go,
	"che-1.0.58-openshift.yaml": che_1_0_58_openshift_yaml,
	"content-repository-2.2.330-openshift.yaml": content_repository_2_2_330_openshift_yaml,
	"fabric8-online.yaml": fabric8_online_yaml,
	"jenkins-openshift-2.2.330-openshift.yaml": jenkins_openshift_2_2_330_openshift_yaml,
}
// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for name := range node.Children {
		rv = append(rv, name)
	}
	return rv, nil
}

type _bintree_t struct {
	Func func() ([]byte, error)
	Children map[string]*_bintree_t
}
var _bintree = &_bintree_t{nil, map[string]*_bintree_t{
	"bindata.go": &_bintree_t{bindata_go, map[string]*_bintree_t{
	}},
	"che-1.0.58-openshift.yaml": &_bintree_t{che_1_0_58_openshift_yaml, map[string]*_bintree_t{
	}},
	"content-repository-2.2.330-openshift.yaml": &_bintree_t{content_repository_2_2_330_openshift_yaml, map[string]*_bintree_t{
	}},
	"fabric8-online.yaml": &_bintree_t{fabric8_online_yaml, map[string]*_bintree_t{
	}},
	"jenkins-openshift-2.2.330-openshift.yaml": &_bintree_t{jenkins_openshift_2_2_330_openshift_yaml, map[string]*_bintree_t{
	}},
}}
