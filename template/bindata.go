package template

import (
	"fmt"
	"strings"
)
var _fabric8_online_che_openshift_yml = []byte(`---
apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: ProjectRequest
  metadata:
    annotations:
      openshift.io/description: ${PROJECT_DESCRIPTION}
      openshift.io/display-name: ${PROJECT_DISPLAYNAME}
      openshift.io/requester: ${PROJECT_REQUESTING_USER}
    labels:
      provider: fabric8
      project: fabric8-online-team
      version: 1.0.79
      group: io.fabric8.online.packages
    name: ${PROJECT_NAME}
- apiVersion: v1
  kind: RoleBindingRestriction
  metadata:
    name: dsaas-user-access
  spec: 
    userrestriction:
      users: 
      - ${PROJECT_USER}
- apiVersion: v1
  kind: LimitRange
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-che
      version: 1.0.81
      group: io.fabric8.online.packages
    name: resource-limits
  spec:
    limits:
    - max:
        cpu: 2930m
        memory: 1500Mi
      min:
        cpu: 29m
        memory: 150Mi
      type: Pod
    - default:
        cpu: "1"
        memory: 512Mi
      defaultRequest:
        cpu: 60m
        memory: 307Mi
      max:
        cpu: 2930m
        memory: 1500Mi
      min:
        cpu: 29m
        memory: 150Mi
      type: Container
    - max:
        storage: 1Gi
      min:
        storage: 1Gi
      type: PersistentVolumeClaim
- apiVersion: v1
  kind: ResourceQuota
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-che
      version: 1.0.81
      group: io.fabric8.online.packages
    name: compute-resources
  spec:
    hard:
      limits.cpu: "4"
      limits.memory: 2Gi
    scopes:
    - NotTerminating
- apiVersion: v1
  kind: ResourceQuota
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-che
      version: 1.0.81
      group: io.fabric8.online.packages
    name: compute-resources-timebound
  spec:
    hard:
      limits.cpu: "4"
      limits.memory: 2Gi
    scopes:
    - Terminating
- apiVersion: v1
  kind: ResourceQuota
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-che
      version: 1.0.81
      group: io.fabric8.online.packages
    name: object-counts
  spec:
    hard:
      persistentvolumeclaims: "2"
      replicationcontrollers: "20"
      secrets: "20"
      services: "5"
- apiVersion: v1
  kind: RoleBinding
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-team
      version: 1.0.79
      group: io.fabric8.online.packages
    name: useradmin
    namespace: ${PROJECT_NAME}
  roleRef:
    name: admin
  subjects:
  - kind: User
    name: ${PROJECT_USER}
  userNames:
  - ${PROJECT_USER}
- apiVersion: v1
  kind: ServiceAccount
  metadata:
    annotations:
      maven.fabric8.io/source-url: jar:file:/home/jenkins/workspace/c8-cd_fabric8-online_master-RZCQJXY66EHCHAKJPPRB7OQ2EYEWQC7JYFR7O4VWUBUVXPQNSF5A@2/apps/che/target/che-1.0.79.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: che
      version: 1.0.79
      group: io.fabric8.online.apps
    name: che
- apiVersion: v1
  kind: Service
  metadata:
    annotations:
      fabric8.io/app-menu: development
      fabric8.io/git-commit: 923a7fa63ae88f89c62d98017ca8bf54b8df89ab
      fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-online.git/apps/che
      fabric8.io/scm-url: http://github.com/fabric8io/fabric8-online/apps/che
      fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-online/master/apps/che/src/main/fabric8/icon.png
      fabric8.io/git-branch: release-v1.0.81
      fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-online.git/apps/che
      fabric8.io/scm-tag: fabric8-online-2.0.1
      maven.fabric8.io/source-url: jar:file:/home/jenkins/workspace/c8-cd_fabric8-online_master-RZCQJXY66EHCHAKJPPRB7OQ2EYEWQC7JYFR7O4VWUBUVXPQNSF5A@2/apps/che/target/che-1.0.81.jar!/META-INF/fabric8/openshift.yml
    labels:
      project: che
      provider: fabric8
      expose: "false"
      version: 1.0.81
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
    annotations:
      maven.fabric8.io/source-url: jar:file:/home/jenkins/workspace/c8-cd_fabric8-online_master-RZCQJXY66EHCHAKJPPRB7OQ2EYEWQC7JYFR7O4VWUBUVXPQNSF5A@2/apps/che/target/che-1.0.81.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: che
      version: 1.0.81
      group: io.fabric8.online.apps
    name: che
  roleRef:
    name: admin
  subjects:
  - kind: ServiceAccount
    name: che
    namespace: ${PROJECT_NAME}    
- apiVersion: v1
  kind: PersistentVolumeClaim
  metadata:
    annotations:
      maven.fabric8.io/source-url: jar:file:/home/jenkins/workspace/c8-cd_fabric8-online_master-RZCQJXY66EHCHAKJPPRB7OQ2EYEWQC7JYFR7O4VWUBUVXPQNSF5A@2/apps/che/target/che-1.0.81.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: che
      version: 1.0.81
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
    annotations:
      maven.fabric8.io/source-url: jar:file:/home/jenkins/workspace/c8-cd_fabric8-online_master-RZCQJXY66EHCHAKJPPRB7OQ2EYEWQC7JYFR7O4VWUBUVXPQNSF5A@2/apps/che/target/che-1.0.81.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: che
      version: 1.0.81
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
    annotations:
      maven.fabric8.io/source-url: jar:file:/home/jenkins/workspace/c8-cd_fabric8-online_master-RZCQJXY66EHCHAKJPPRB7OQ2EYEWQC7JYFR7O4VWUBUVXPQNSF5A@2/apps/che/target/che-1.0.81.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: che
      version: 1.0.81
      group: io.fabric8.online.apps
    name: che
  data:
    hostname-http: che-${PROJECT_NAME}.d800.free-int.openshiftapps.com
    workspace-storage: /home/user/che/workspaces
    workspace-storage-create-folders: "false"
    local-conf-dir: /etc/conf
    openshift-serviceaccountname: che
    che-server-evaluation-strategy: single-port
    log-level: INFO
    docker-connector: openshift
    port: "8080"
    remote-debugging-enabled: "false"
    che-oauth-github-forceactivation: "true"
    workspaces-memory-limit: 1300Mi
    workspaces-memory-request: 500Mi
    enable-workspaces-autostart: "false"
    che-server-java-opts: -XX:+UseSerialGC -XX:MinHeapFreeRatio=20 -XX:MaxHeapFreeRatio=40 -XX:MaxRAM=700m -Xms256m
    che-workspaces-java-opts: -XX:+UseSerialGC -XX:MinHeapFreeRatio=20 -XX:MaxHeapFreeRatio=40 -XX:MaxRAM=1300m -Xms256m
    che-openshift-secure-routes: "true"
    che-secure-external-urls: "true"
- apiVersion: v1
  kind: ConfigMap
  metadata:
    labels:
      fabric8.io/kind: package
      provider: fabric8.io
      version: 1.0.81
      project: fabric8-online-che
      group: io.fabric8.online.packages
    name: fabric8-online-che
  data:
    metadata-url: http://central.maven.org/maven2/io/fabric8/online/packages/fabric8-online-che/maven-metadata.xml
    package-url-prefix: http://central.maven.org/maven2/io/fabric8/online/packages/fabric8-online-che/%[1]s/fabric8-online-che-%[1]s-
- apiVersion: v1
  kind: DeploymentConfig
  metadata:
    annotations:
      fabric8.io/git-commit: 923a7fa63ae88f89c62d98017ca8bf54b8df89ab
      fabric8.io/metrics-path: dashboard/file/kubernetes-pods.json/?var-project=che&var-version=1.0.81
      fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-online.git/apps/che
      fabric8.io/scm-url: http://github.com/fabric8io/fabric8-online/apps/che
      fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-online/master/apps/che/src/main/fabric8/icon.png
      fabric8.io/git-branch: release-v1.0.81
      fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-online.git/apps/che
      fabric8.io/scm-tag: fabric8-online-2.0.1
      maven.fabric8.io/source-url: jar:file:/home/jenkins/workspace/c8-cd_fabric8-online_master-RZCQJXY66EHCHAKJPPRB7OQ2EYEWQC7JYFR7O4VWUBUVXPQNSF5A@2/apps/che/target/che-1.0.81.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: che
      version: 1.0.81
      group: io.fabric8.online.apps
    name: che
  spec:
    replicas: 1
    selector:
      project: che
      provider: fabric8
      version: 1.0.81
      group: io.fabric8.online.apps
    template:
      metadata:
        annotations:
          fabric8.io/git-commit: 923a7fa63ae88f89c62d98017ca8bf54b8df89ab
          fabric8.io/metrics-path: dashboard/file/kubernetes-pods.json/?var-project=che&var-version=1.0.81
          fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-online.git/apps/che
          fabric8.io/scm-url: http://github.com/fabric8io/fabric8-online/apps/che
          fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-online/master/apps/che/src/main/fabric8/icon.png
          fabric8.io/git-branch: release-v1.0.81
          fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-online.git/apps/che
          fabric8.io/scm-tag: fabric8-online-2.0.1
        labels:
          provider: fabric8
          project: che
          version: 1.0.81
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
          - name: CHE_OAUTH_GITHUB_FORCEACTIVATION
            valueFrom:
              configMapKeyRef:
                key: che-oauth-github-forceactivation
                name: che
          - name: CHE_OPENSHIFT_WORKSPACE_MEMORY_OVERRIDE
            valueFrom:
              configMapKeyRef:
                key: workspaces-memory-limit
                name: che
          - name: CHE_OPENSHIFT_WORKSPACE_MEMORY_REQUEST
            valueFrom:
              configMapKeyRef:
                key: workspaces-memory-request
                name: che
          - name: CHE_WORKSPACE_AUTO_START
            valueFrom:
              configMapKeyRef:
                key: enable-workspaces-autostart
                name: che
          - name: JAVA_OPTS
            valueFrom:
              configMapKeyRef:
                key: che-server-java-opts
                name: che
          - name: CHE_WORKSPACE_JAVA_OPTIONS
            valueFrom:
              configMapKeyRef:
                key: che-workspaces-java-opts
                name: che
          - name: CHE_OPENSHIFT_SECURE_ROUTES
            valueFrom:
              configMapKeyRef:
                key: che-openshift-secure-routes
                name: che
          - name: CHE_DOCKER_SERVER__EVALUATION__STRATEGY_SECURE_EXTERNAL_URLS
            valueFrom:
              configMapKeyRef:
                key: che-secure-external-urls
                name: che
          image: rhche/che-server:3b7e9b9
          imagePullPolicy: IfNotPresent
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
          resources:
            limits:
              memory: 700Mi
            requests:
              memory: 256Mi
          volumeMounts:
          - mountPath: /data
            name: che-data-volume
        serviceAccountName: che
        volumes:
        - name: che-data-volume
          persistentVolumeClaim:
            claimName: che-data-volume
    triggers:
    - type: ConfigChange
- apiVersion: v1
  kind: Route
  metadata:
    annotations:
      maven.fabric8.io/source-url: jar:file:/home/jenkins/workspace/c8-cd_fabric8-online_master-RZCQJXY66EHCHAKJPPRB7OQ2EYEWQC7JYFR7O4VWUBUVXPQNSF5A@2/apps/che/target/che-1.0.81.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: che
      version: 1.0.81
      group: io.fabric8.online.apps
    name: che
  spec:
    tls:
      termination: edge
    to:
      kind: Service
      name: che-host`)

func fabric8_online_che_openshift_yml() ([]byte, error) {
	return _fabric8_online_che_openshift_yml, nil
}

var _fabric8_online_jenkins_openshift_yml = []byte(`---
apiVersion: v1
kind: Template
metadata:
  labels:
    provider: fabric8
    project: fabric8-online-jenkins
    version: 1.0.81
    group: io.fabric8.online.packages
  name: fabric8-online-jenkins
objects:
- apiVersion: v1
  kind: ProjectRequest
  metadata:
    annotations:
      openshift.io/description: ${PROJECT_DESCRIPTION}
      openshift.io/display-name: ${PROJECT_DISPLAYNAME}
      openshift.io/requester: ${PROJECT_REQUESTING_USER}
    labels:
      provider: fabric8
      project: fabric8-online-team
      version: 1.0.79
      group: io.fabric8.online.packages
    name: ${PROJECT_NAME}
- apiVersion: v1
  kind: RoleBindingRestriction
  metadata:
    name: dsaas-user-access
  spec: 
    userrestriction:
      users:
      - ${PROJECT_USER}
- apiVersion: v1
  kind: LimitRange
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-jenkins
      version: 1.0.81
      group: io.fabric8.online.packages
    name: resource-limits
  spec:
    limits:
    - max:
        cpu: "3"
        memory: 1536Mi
      min:
        cpu: 17m
        memory: 90Mi
      type: Pod
    - default:
        cpu: "1"
        memory: 512Mi
      defaultRequest:
        cpu: 60m
        memory: 307Mi
      max:
        cpu: "3"
        memory: 1536Mi
      min:
        cpu: 17m
        memory: 90Mi
      type: Container
    - max:
        storage: 1Gi
      min:
        storage: 1Gi
      type: PersistentVolumeClaim
- apiVersion: v1
  kind: ResourceQuota
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-jenkins
      version: 1.0.81
      group: io.fabric8.online.packages
    name: compute-resources
  spec:
    hard:
      limits.cpu: "4"
      limits.memory: 2Gi
    scopes:
    - NotTerminating
- apiVersion: v1
  kind: ResourceQuota
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-jenkins
      version: 1.0.81
      group: io.fabric8.online.packages
    name: compute-resources-timebound
  spec:
    hard:
      limits.cpu: "3"
      limits.memory: 1536Mi
    scopes:
    - Terminating
- apiVersion: v1
  kind: ResourceQuota
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-jenkins
      version: 1.0.81
      group: io.fabric8.online.packages
    name: object-counts
  spec:
    hard:
      persistentvolumeclaims: "3"
      replicationcontrollers: "20"
      secrets: "40"
      services: "5"
- apiVersion: v1
  kind: Secret
  metadata:
    annotations:
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/jenkins-openshift/3.0.5/jenkins-openshift-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
    name: jenkins-docker-cfg
  data:
    config.json: ""
  type: fabric8.io/jenkins-docker-cfg
- apiVersion: v1
  kind: Secret
  metadata:
    annotations:
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/jenkins-openshift/3.0.5/jenkins-openshift-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
    name: jenkins-git-ssh
  data:
    ssh-key: ""
    ssh-key.pub: ""
  type: fabric8.io/jenkins-git-ssh
- apiVersion: v1
  kind: Secret
  metadata:
    annotations:
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/jenkins-openshift/3.0.5/jenkins-openshift-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
    name: jenkins-hub-api-token
  data:
    hub: ""
  type: fabric8.io/jenkins-hub-api-token
- apiVersion: v1
  kind: Secret
  metadata:
    annotations:
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/jenkins-openshift/3.0.5/jenkins-openshift-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
    name: jenkins-maven-settings
  data:
    settings.xml: PHNldHRpbmdzPg0KICA8IS0tIHNldHMgdGhlIGxvY2FsIG1hdmVuIHJlcG9zaXRvcnkgb3V0c2lkZSBvZiB0aGUgfi8ubTIgZm9sZGVyIGZvciBlYXNpZXIgbW91bnRpbmcgb2Ygc2VjcmV0cyBhbmQgcmVwbyAtLT4NCiAgPGxvY2FsUmVwb3NpdG9yeT4ke3VzZXIuaG9tZX0vLm12bnJlcG9zaXRvcnk8L2xvY2FsUmVwb3NpdG9yeT4NCiAgPG1pcnJvcnM+DQogICAgPG1pcnJvcj4NCiAgICAgIDxpZD5uZXh1czwvaWQ+DQogICAgICA8bWlycm9yT2Y+ZXh0ZXJuYWw6KjwvbWlycm9yT2Y+DQogICAgICA8dXJsPmh0dHA6Ly9jZW50cmFsLm1hdmVuLm9yZy9tYXZlbjIvPC91cmw+DQogICAgPC9taXJyb3I+DQogIDwvbWlycm9ycz4NCg0KICA8IS0tIGxldHMgZGlzYWJsZSB0aGUgZG93bmxvYWQgcHJvZ3Jlc3MgaW5kaWNhdG9yIHRoYXQgZmlsbHMgdXAgbG9ncyAtLT4NCiAgPGludGVyYWN0aXZlTW9kZT5mYWxzZTwvaW50ZXJhY3RpdmVNb2RlPg0KDQogIDxzZXJ2ZXJzPg0KICAgIDxzZXJ2ZXI+DQogICAgICA8aWQ+bG9jYWwtbmV4dXM8L2lkPg0KICAgICAgPHVzZXJuYW1lPmFkbWluPC91c2VybmFtZT4NCiAgICAgIDxwYXNzd29yZD5hZG1pbjEyMzwvcGFzc3dvcmQ+DQogICAgPC9zZXJ2ZXI+DQogICAgPHNlcnZlcj4NCiAgICAgIDxpZD5uZXh1czwvaWQ+DQogICAgICA8dXNlcm5hbWU+YWRtaW48L3VzZXJuYW1lPg0KICAgICAgPHBhc3N3b3JkPmFkbWluMTIzPC9wYXNzd29yZD4NCiAgICA8L3NlcnZlcj4NCiAgICA8c2VydmVyPg0KICAgICAgPGlkPm9zcy1zb25hdHlwZS1zdGFnaW5nPC9pZD4NCiAgICAgIDx1c2VybmFtZT48L3VzZXJuYW1lPg0KICAgICAgPHBhc3N3b3JkPjwvcGFzc3dvcmQ+DQogICAgPC9zZXJ2ZXI+DQogIDwvc2VydmVycz4NCg0KICA8cHJvZmlsZXM+DQogICAgPHByb2ZpbGU+DQogICAgICA8aWQ+bmV4dXM8L2lkPg0KICAgICAgPHByb3BlcnRpZXM+DQogICAgICAgIDxhbHREZXBsb3ltZW50UmVwb3NpdG9yeT5sb2NhbC1uZXh1czo6ZGVmYXVsdDo6aHR0cDovL2NvbnRlbnQtcmVwb3NpdG9yeS9jb250ZW50L3JlcG9zaXRvcmllcy9zdGFnaW5nLzwvYWx0RGVwbG95bWVudFJlcG9zaXRvcnk+DQogICAgICAgIDxhbHRSZWxlYXNlRGVwbG95bWVudFJlcG9zaXRvcnk+bG9jYWwtbmV4dXM6OmRlZmF1bHQ6Omh0dHA6Ly9jb250ZW50LXJlcG9zaXRvcnkvY29udGVudC9yZXBvc2l0b3JpZXMvc3RhZ2luZy88L2FsdFJlbGVhc2VEZXBsb3ltZW50UmVwb3NpdG9yeT4NCiAgICAgICAgPGFsdFNuYXBzaG90RGVwbG95bWVudFJlcG9zaXRvcnk+bG9jYWwtbmV4dXM6OmRlZmF1bHQ6Omh0dHA6Ly9jb250ZW50LXJlcG9zaXRvcnkvY29udGVudC9yZXBvc2l0b3JpZXMvc25hcHNob3RzLzwvYWx0U25hcHNob3REZXBsb3ltZW50UmVwb3NpdG9yeT4NCiAgICAgIDwvcHJvcGVydGllcz4NCiAgICAgIDxyZXBvc2l0b3JpZXM+DQogICAgICAgIDxyZXBvc2l0b3J5Pg0KICAgICAgICAgIDxpZD5jZW50cmFsPC9pZD4NCiAgICAgICAgICA8dXJsPmh0dHA6Ly9jZW50cmFsPC91cmw+DQogICAgICAgICAgPHJlbGVhc2VzPjxlbmFibGVkPnRydWU8L2VuYWJsZWQ+PC9yZWxlYXNlcz4NCiAgICAgICAgICA8c25hcHNob3RzPjxlbmFibGVkPnRydWU8L2VuYWJsZWQ+PC9zbmFwc2hvdHM+DQogICAgICAgIDwvcmVwb3NpdG9yeT4NCiAgICAgIDwvcmVwb3NpdG9yaWVzPg0KICAgICAgPHBsdWdpblJlcG9zaXRvcmllcz4NCiAgICAgICAgPHBsdWdpblJlcG9zaXRvcnk+DQogICAgICAgICAgPGlkPmNlbnRyYWw8L2lkPg0KICAgICAgICAgIDx1cmw+aHR0cDovL2NlbnRyYWw8L3VybD4NCiAgICAgICAgICA8cmVsZWFzZXM+PGVuYWJsZWQ+dHJ1ZTwvZW5hYmxlZD48L3JlbGVhc2VzPg0KICAgICAgICAgIDxzbmFwc2hvdHM+PGVuYWJsZWQ+dHJ1ZTwvZW5hYmxlZD48L3NuYXBzaG90cz4NCiAgICAgICAgPC9wbHVnaW5SZXBvc2l0b3J5Pg0KICAgICAgPC9wbHVnaW5SZXBvc2l0b3JpZXM+DQogICAgPC9wcm9maWxlPg0KICAgIDxwcm9maWxlPg0KICAgICAgPGlkPnJlbGVhc2U8L2lkPg0KICAgICAgPHByb3BlcnRpZXM+DQogICAgICAgIDxncGcuZXhlY3V0YWJsZT5ncGc8L2dwZy5leGVjdXRhYmxlPg0KICAgICAgICA8Z3BnLnBhc3NwaHJhc2U+bXlzZWNyZXRwYXNzcGhyYXNlPC9ncGcucGFzc3BocmFzZT4NCiAgICAgIDwvcHJvcGVydGllcz4NCiAgICA8L3Byb2ZpbGU+DQogIDwvcHJvZmlsZXM+DQogIDxhY3RpdmVQcm9maWxlcz4NCiAgICA8IS0tbWFrZSB0aGUgcHJvZmlsZSBhY3RpdmUgYWxsIHRoZSB0aW1lIC0tPg0KICAgIDxhY3RpdmVQcm9maWxlPm5leHVzPC9hY3RpdmVQcm9maWxlPg0KICA8L2FjdGl2ZVByb2ZpbGVzPg0KPC9zZXR0aW5ncz4NCg==
  type: fabric8.io/secret-maven-settings
- apiVersion: v1
  kind: Secret
  metadata:
    annotations:
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/jenkins-openshift/3.0.5/jenkins-openshift-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
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
    annotations:
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/jenkins-openshift/3.0.5/jenkins-openshift-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
    name: jenkins-ssh-config
  data:
    config: ""
  type: fabric8.io/jenkins-ssh-config
- apiVersion: v1
  kind: ServiceAccount
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-jenkins
      version: 1.0.81
      group: io.fabric8.online.packages
    name: cd-bot
- apiVersion: v1
  kind: ServiceAccount
  metadata:
    annotations:
      serviceaccounts.openshift.io/oauth-redirectreference.jenkins: '{"kind":"OAuthRedirectReference","apiVersion":"v1","reference":{"kind":"Route","name":"jenkins"}}'
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/jenkins-openshift/3.0.5/jenkins-openshift-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
    name: jenkins
- apiVersion: v1
  kind: Service
  metadata:
    annotations:
      fabric8.io/git-commit: 923a7fa63ae88f89c62d98017ca8bf54b8df89ab
      fabric8.io/git-branch: release-v1.0.81
      fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-online.git/apps/bayesian-link
      fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-online.git/apps/bayesian-link
      fabric8.io/scm-tag: fabric8-online-2.0.1
      fabric8.io/scm-url: http://github.com/fabric8io/fabric8-online/apps/bayesian-link
      maven.fabric8.io/source-url: jar:file:/home/jenkins/workspace/c8-cd_fabric8-online_master-RZCQJXY66EHCHAKJPPRB7OQ2EYEWQC7JYFR7O4VWUBUVXPQNSF5A@2/apps/bayesian-link/target/bayesian-link-1.0.81.jar!/META-INF/fabric8/openshift.yml
    labels:
      expose: "false"
      provider: fabric8
      project: bayesian-link
      version: 1.0.81
      group: io.fabric8.online.apps
    name: bayesian-link
  spec:
    externalName: bayesian
    ports:
    - port: 80
    selector:
      project: bayesian-link
      provider: fabric8
      group: io.fabric8.online.apps
    type: ExternalName
- apiVersion: v1
  kind: Service
  metadata:
    annotations:
      fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-devops/master/content-repository/src/main/fabric8/icon.svg
      prometheus.io/port: "9180"
      prometheus.io/scrape: "true"
      fabric8.io/git-commit: 6c85b448aab3c238057ba665e40673c22f80a93a
      fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-team-components.git/content-repository
      fabric8.io/scm-url: http://github.com/fabric8io/fabric8-team-components/content-repository
      fabric8.io/git-branch: release-v3.0.5
      fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-team-components.git/content-repository
      fabric8.io/scm-tag: fabric8-team-components-1.0.0
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/content-repository/3.0.5/content-repository-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: content-repository
      version: 3.0.5
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
      group: io.fabric8.fabric8-team-components.apps
- apiVersion: v1
  kind: Service
  metadata:
    annotations:
      fabric8.io/app-menu: development
      fabric8.io/git-commit: 6c85b448aab3c238057ba665e40673c22f80a93a
      fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-team-components.git/jenkins-openshift
      fabric8.io/scm-url: http://github.com/fabric8io/fabric8-team-components/jenkins-openshift
      fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-team-components/master/jenkins-openshift/src/main/fabric8/icon.svg
      fabric8.io/git-branch: release-v3.0.5
      fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-team-components.git/jenkins-openshift
      fabric8.io/scm-tag: fabric8-team-components-1.0.0
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/jenkins-openshift/3.0.5/jenkins-openshift-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      project: jenkins
      provider: fabric8
      expose: "false"
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
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
      group: io.fabric8.fabric8-team-components.apps
- apiVersion: v1
  kind: Service
  metadata:
    annotations:
      fabric8.io/git-commit: 6c85b448aab3c238057ba665e40673c22f80a93a
      fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-team-components.git/jenkins-openshift
      fabric8.io/scm-url: http://github.com/fabric8io/fabric8-team-components/jenkins-openshift
      fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-team-components/master/jenkins-openshift/src/main/fabric8/icon.svg
      fabric8.io/git-branch: release-v3.0.5
      fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-team-components.git/jenkins-openshift
      fabric8.io/scm-tag: fabric8-team-components-1.0.0
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/jenkins-openshift/3.0.5/jenkins-openshift-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      project: jenkins
      provider: fabric8
      expose: "false"
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
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
      group: io.fabric8.fabric8-team-components.apps
- apiVersion: v1
  kind: RoleBinding
  metadata:
    annotations:
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/jenkins-openshift/3.0.5/jenkins-openshift-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
    name: edit-jenkins
  roleRef:
    name: edit
  subjects:
  - kind: ServiceAccount
    name: jenkins
    namespace: ${PROJECT_NAME}
- apiVersion: v1
  kind: RoleBinding
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-jenkins
      version: 1.0.81
      group: io.fabric8.online.packages
    name: view
  roleRef:
    name: view
  subjects:
  - kind: User
    name: ${PROJECT_USER}
  userNames:
  - ${PROJECT_USER}
- apiVersion: v1
  kind: RoleBinding
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-jenkins
      version: 1.0.81
      group: io.fabric8.online.packages
    name: view-cd-bot
  roleRef:
    name: view
  subjects:
  - kind: ServiceAccount
    name: cd-bot
    namespace: ${PROJECT_NAME}
- apiVersion: v1
  kind: RoleBinding
  metadata:
    annotations:
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/jenkins-openshift/3.0.5/jenkins-openshift-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
    name: view-jenkins
  roleRef:
    name: view
  subjects:
  - kind: ServiceAccount
    name: jenkins
    namespace: ${PROJECT_NAME}
- apiVersion: v1
  kind: PersistentVolumeClaim
  metadata:
    annotations:
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/content-repository/3.0.5/content-repository-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: content-repository
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
    name: content-repository
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
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/jenkins-openshift/3.0.5/jenkins-openshift-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
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
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/jenkins-openshift/3.0.5/jenkins-openshift-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
    name: jenkins-mvn-local-repo
  spec:
    accessModes:
    - ReadWriteMany
    resources:
      requests:
        storage: 1Gi
- apiVersion: v1
  kind: ConfigMap
  metadata:
    labels:
      fabric8.io/kind: package
      provider: fabric8.io
      version: 1.0.81
      project: fabric8-online-jenkins
      group: io.fabric8.online.packages
    name: fabric8-online-jenkins
  data:
    metadata-url: http://central.maven.org/maven2/io/fabric8/online/packages/fabric8-online-jenkins/maven-metadata.xml
    package-url-prefix: http://central.maven.org/maven2/io/fabric8/online/packages/fabric8-online-jenkins/%[1]s/fabric8-online-jenkins-%[1]s-
- apiVersion: v1
  kind: DeploymentConfig
  metadata:
    annotations:
      fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-devops/master/content-repository/src/main/fabric8/icon.svg
      fabric8.io/git-commit: 6c85b448aab3c238057ba665e40673c22f80a93a
      fabric8.io/metrics-path: dashboard/file/kubernetes-pods.json/?var-project=content-repository&var-version=3.0.5
      fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-team-components.git/content-repository
      fabric8.io/scm-url: http://github.com/fabric8io/fabric8-team-components/content-repository
      fabric8.io/git-branch: release-v3.0.5
      fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-team-components.git/content-repository
      fabric8.io/scm-tag: fabric8-team-components-1.0.0
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/content-repository/3.0.5/content-repository-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: content-repository
      version: 3.0.5
      group: io.fabric8.devops.apps
    name: content-repository
  spec:
    replicas: 1
    selector:
      project: content-repository
      provider: fabric8
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
    template:
      metadata:
        annotations:
          fabric8.io/git-commit: 6c85b448aab3c238057ba665e40673c22f80a93a
          fabric8.io/metrics-path: dashboard/file/kubernetes-pods.json/?var-project=content-repository&var-version=3.0.5
          fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-team-components.git/content-repository
          fabric8.io/scm-url: http://github.com/fabric8io/fabric8-team-components/content-repository
          fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-devops/master/content-repository/src/main/fabric8/icon.svg
          fabric8.io/git-branch: release-v3.0.5
          fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-team-components.git/content-repository
          fabric8.io/scm-tag: fabric8-team-components-1.0.0
        labels:
          provider: fabric8
          project: content-repository
          version: 3.0.5
          group: io.fabric8.fabric8-team-components.apps
      spec:
        containers:
        - env:
          - name: KUBERNETES_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
          image: fabric8/caddy-server:v9274a15
          imagePullPolicy: IfNotPresent
          name: content-repository
          ports:
          - containerPort: 8080
            name: http
          - containerPort: 9180
            name: prometheus
          resources:
            limits:
              cpu: "1"
              memory: 512Mi
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
  kind: DeploymentConfig
  metadata:
    annotations:
      fabric8.io/git-commit: 6c85b448aab3c238057ba665e40673c22f80a93a
      fabric8.io/metrics-path: dashboard/file/kubernetes-pods.json/?var-project=jenkins-openshift&var-version=3.0.5
      fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-team-components.git/jenkins-openshift
      fabric8.io/scm-url: http://github.com/fabric8io/fabric8-team-components/jenkins-openshift
      fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-team-components/master/jenkins-openshift/src/main/fabric8/icon.svg
      fabric8.io/git-branch: release-v3.0.5
      fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-team-components.git/jenkins-openshift
      fabric8.io/scm-tag: fabric8-team-components-1.0.0
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/jenkins-openshift/3.0.5/jenkins-openshift-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      fabric8.io/type: preview
      provider: fabric8
      project: jenkins-openshift
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
    name: jenkins
  spec:
    replicas: 1
    selector:
      project: jenkins-openshift
      provider: fabric8
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
    template:
      metadata:
        annotations:
          fabric8.io/git-commit: 6c85b448aab3c238057ba665e40673c22f80a93a
          fabric8.io/metrics-path: dashboard/file/kubernetes-pods.json/?var-project=jenkins-openshift&var-version=3.0.5
          fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-team-components.git/jenkins-openshift
          fabric8.io/scm-url: http://github.com/fabric8io/fabric8-team-components/jenkins-openshift
          fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-team-components/master/jenkins-openshift/src/main/fabric8/icon.svg
          fabric8.io/git-branch: release-v3.0.5
          fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-team-components.git/jenkins-openshift
          fabric8.io/scm-tag: fabric8-team-components-1.0.0
        labels:
          provider: fabric8
          project: jenkins-openshift
          version: 3.0.5
          group: io.fabric8.fabric8-team-components.apps
      spec:
        containers:
        - env:
          - name: PROJECT_NAMESPACE
            value: ${PROJECT_NAMESPACE}
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
          image: fabric8/jenkins-openshift:v57fc8be
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
              memory: 1Gi
              cpu: "2"
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
    annotations:
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/content-repository/3.0.5/content-repository-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: content-repository
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
    name: content-repository
  spec:
    tls:
      insecureEdgeTerminationPolicy: Redirect
      termination: edge
    to:
      kind: Service
      name: content-repository
- apiVersion: v1
  kind: Route
  metadata:
    annotations:
      maven.fabric8.io/source-url: jar:file:/root/.mvnrepository/io/fabric8/fabric8-team-components/apps/jenkins-openshift/3.0.5/jenkins-openshift-3.0.5.jar!/META-INF/fabric8/openshift.yml
    labels:
      provider: fabric8
      project: jenkins-openshift
      version: 3.0.5
      group: io.fabric8.fabric8-team-components.apps
    name: jenkins
  spec:
    tls:
      insecureEdgeTerminationPolicy: Redirect
      termination: edge
    to:
      kind: Service
      name: jenkins
parameters:
- name: PROJECT_USER
  value: developer
- name: PROJECT_NAMESPACE`)

func fabric8_online_jenkins_openshift_yml() ([]byte, error) {
	return _fabric8_online_jenkins_openshift_yml, nil
}

var _fabric8_online_project_openshift_yml = []byte(`---
apiVersion: v1
kind: Template
metadata:
  labels:
    provider: fabric8
    project: fabric8-online-jenkins
    version: 1.0.66
    group: io.fabric8.online.packages
  name: fabric8-online-jenkins
objects:
- apiVersion: v1
  kind: ProjectRequest
  metadata:
    annotations:
      openshift.io/description: ${PROJECT_DESCRIPTION}
      openshift.io/display-name: ${PROJECT_DISPLAYNAME}
      openshift.io/requester: ${PROJECT_REQUESTING_USER}
    labels:
      provider: fabric8
      project: fabric8-online-team
      version: 1.0.66
      group: io.fabric8.online.packages
    name: ${PROJECT_NAME}
- apiVersion: v1
  kind: RoleBindingRestriction
  metadata:
    name: dsaas-user-access
  spec: 
    userrestriction:
      users: 
      - ${PROJECT_USER}
- apiVersion: v1
  kind: RoleBinding
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-jenkins
      version: 1.0.66
      group: io.fabric8.online.packages
    name: useradmin
  roleRef:
    name: admin
  subjects:
  - kind: User
    name: ${PROJECT_USER}
  userNames:
  - ${PROJECT_USER}
`)

func fabric8_online_project_openshift_yml() ([]byte, error) {
	return _fabric8_online_project_openshift_yml, nil
}

var _fabric8_online_team_colaborators_yml = []byte(`---
apiVersion: v1
kind: Template
metadata:
objects:
- apiVersion: v1
  kind: RoleBindingRestriction
  metadata:
    name: dsaas-access-users
  spec: 
    userrestriction:
      users: 
      - devtools-sre
      - system:serviceaccount:${PROJECT_NAME}-jenkins:jenkins
- apiVersion: v1
  kind: RoleBindingRestriction
  metadata:
    name: dsaas-access-groups
  spec: 
    grouprestriction:
      groups: 
      - system:serviceaccounts:${PROJECT_NAME}-jenkins
- apiVersion: v1
  kind: RoleBindingRestriction
  metadata:
    name: dsaas-access-sa-ns
  spec: 
    serviceaccountrestriction:
      namespaces: 
      - ${PROJECT_NAME}-jenkins
parameters:
- name: PROJECT_NAME`)

func fabric8_online_team_colaborators_yml() ([]byte, error) {
	return _fabric8_online_team_colaborators_yml, nil
}

var _fabric8_online_team_openshift_yml = []byte(`---
apiVersion: v1
kind: Template
metadata:
  labels:
    provider: fabric8
    project: fabric8-online-team
    version: 1.0.74
    group: io.fabric8.online.packages
  name: fabric8-online-team
objects:
- apiVersion: v1
  kind: ProjectRequest
  metadata:
    annotations:
      openshift.io/description: ${PROJECT_DESCRIPTION}
      openshift.io/display-name: ${PROJECT_DISPLAYNAME}
      openshift.io/requester: ${PROJECT_REQUESTING_USER}
    labels:
      provider: fabric8
      project: fabric8-online-team
      version: 1.0.74
      group: io.fabric8.online.packages
    name: ${PROJECT_NAME}
- apiVersion: v1
  kind: Service
  metadata:
    annotations:
      fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-devops/master/content-repository/src/main/fabric8/icon.svg
      fabric8.io/git-commit: ff04d40357d4b680ee0c685b6ea5fb247f0e5a86
      fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-online.git/packages/fabric8-online-team
      fabric8.io/scm-url: http://github.com/fabric8io/fabric8-online/packages/fabric8-online-team
      fabric8.io/git-branch: release-v1.0.74
      fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-online.git/packages/fabric8-online-team
      fabric8.io/scm-tag: fabric8-online-2.0.1
    labels:
      expose: "true"
      provider: fabric8
      project: fabric8-online-team
      version: 1.0.74
      group: io.fabric8.online.packages
    name: content-repository
  spec:
    externalName: content-repository.${PROJECT_NAME}-jenkins
    ports:
    - port: 80
    selector:
      project: fabric8-online-team
      provider: fabric8
      group: io.fabric8.online.packages
    type: ExternalName
- apiVersion: v1
  kind: Service
  metadata:
    annotations:
      fabric8.io/app-menu: development
      fabric8.io/git-commit: 2626c7c3ef02e7dcfc54d9cd7b9359b66ecf269c
      fabric8.io/scm-con-url: scm:git:git@github.com:fabric8io/fabric8-devops.git/jenkins-openshift
      fabric8.io/scm-url: http://github.com/fabric8io/fabric8-devops/jenkins-openshift
      fabric8.io/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-devops/master/jenkins-openshift/src/main/fabric8/icon.svg
      fabric8.io/git-branch: release-v2.2.329
      fabric8.io/scm-devcon-url: scm:git:git@github.com:fabric8io/fabric8-devops.git/jenkins-openshift
      fabric8.io/scm-tag: fabric8-devops-2.0.1
    labels:
      project: jenkins
      provider: fabric8
      expose: "false"
      version: 2.2.329
      group: io.fabric8.devops.apps
    name: jenkins
  spec:
    externalName: jenkins.${PROJECT_NAME}-jenkins
    ports:
    - name: http
      port: 80
    selector:
      project: fabric8-online-team
      provider: fabric8
      group: io.fabric8.online.packages
    type: ExternalName
- apiVersion: v1
  kind: ConfigMap
  metadata:
    annotations:
      description: Defines the environments used by your Continuous Delivery pipelines.
      fabric8.console/iconUrl: https://cdn.rawgit.com/fabric8io/fabric8-console/master/app-kubernetes/src/main/fabric8/icon.svg
    labels:
      kind: environments
      provider: fabric8
      project: fabric8-online-team
      version: 1.0.74
      group: io.fabric8.online.packages
    name: fabric8-environments
  data:
    test: |-
      name: Test
      namespace: ${PROJECT_NAME}-test
      order: 0
    stage: |-
      name: Stage
      namespace: ${PROJECT_NAME}-stage
      order: 1
    run: |-
      name: Run
      namespace: ${PROJECT_NAME}-run
      order: 2
- apiVersion: v1
  kind: ConfigMap
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-team
      version: 1.0.74
      group: io.fabric8.online.packages
    name: fabric8-pipelines
  data:
    ci-branch-patterns: '- PR-.*'
    cd-branch-patterns: '- master'
parameters:
- name: PROJECT_NAME
  value: myproject
- name: PROJECT_ADMIN_USER
  value: developer
- name: PROJECT_REQUESTING_USER
  value: system:admin
- name: PROJECT_DISPLAYNAME
- name: PROJECT_DESCRIPTION`)

func fabric8_online_team_openshift_yml() ([]byte, error) {
	return _fabric8_online_team_openshift_yml, nil
}

var _fabric8_online_team_rolebindings_yml = []byte(`---
apiVersion: v1
kind: Template
metadata:
objects:
- apiVersion: v1
  kind: RoleBinding
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-team
      version: 1.0.74
      group: io.fabric8.online.packages
    name: dsaas-admin
    namespace: ${PROJECT_NAME}
  roleRef:
    name: admin
  subjects:
  - kind: User
    name: ${PROJECT_ADMIN_USER}
  userNames:
  - ${PROJECT_ADMIN_USER}
- apiVersion: v1
  kind: RoleBinding
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-team
      version: 1.0.74
      group: io.fabric8.online.packages
    name: jenkins-edit
    namespace: ${PROJECT_NAME}
  roleRef:
    name: edit
  subjects:
  - kind: ServiceAccount
    name: jenkins
  userNames:
  - system:serviceaccount:${PROJECT_NAME}-jenkins:jenkins
- apiVersion: v1
  kind: RoleBinding
  metadata:
    labels:
      provider: fabric8
      project: fabric8-online-team
      version: 1.0.74
      group: io.fabric8.online.packages
    name: jenkins-view
    namespace: ${PROJECT_NAME}
  roleRef:
    name: view
  subjects:
  - kind: ServiceAccount
    name: jenkins
  userNames:
  - system:serviceaccount:${PROJECT_NAME}-jenkins:jenkins
`)

func fabric8_online_team_rolebindings_yml() ([]byte, error) {
	return _fabric8_online_team_rolebindings_yml, nil
}

var _generate_go = []byte(`//xgo:generate sh -c "curl http://central.maven.org/maven2/io/fabric8/online/packages/fabric8-online-team/$TEAM_VERSION/fabric8-online-team-$TEAM_VERSION-openshift.yml | grep -v storage-class > fabric8-online-team-openshift.yml"
//xgo:generate sh -c "curl http://central.maven.org/maven2/io/fabric8/online/packages/fabric8-online-jenkins/$TEAM_VERSION/fabric8-online-jenkins-$TEAM_VERSION-openshift.yml | grep -v storage-class > fabric8-online-jenkins-openshift.yml"
//xgo:generate sh -c "curl http://central.maven.org/maven2/io/fabric8/online/packages/fabric8-online-che/$TEAM_VERSION/fabric8-online-che-$TEAM_VERSION-openshift.yml | grep -v storage-class > fabric8-online-che-openshift.yml"
package template
`)

func generate_go() ([]byte, error) {
	return _generate_go, nil
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
	"fabric8-online-che-openshift.yml": fabric8_online_che_openshift_yml,
	"fabric8-online-jenkins-openshift.yml": fabric8_online_jenkins_openshift_yml,
	"fabric8-online-project-openshift.yml": fabric8_online_project_openshift_yml,
	"fabric8-online-team-colaborators.yml": fabric8_online_team_colaborators_yml,
	"fabric8-online-team-openshift.yml": fabric8_online_team_openshift_yml,
	"fabric8-online-team-rolebindings.yml": fabric8_online_team_rolebindings_yml,
	"generate.go": generate_go,
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
	"fabric8-online-che-openshift.yml": &_bintree_t{fabric8_online_che_openshift_yml, map[string]*_bintree_t{
	}},
	"fabric8-online-jenkins-openshift.yml": &_bintree_t{fabric8_online_jenkins_openshift_yml, map[string]*_bintree_t{
	}},
	"fabric8-online-project-openshift.yml": &_bintree_t{fabric8_online_project_openshift_yml, map[string]*_bintree_t{
	}},
	"fabric8-online-team-colaborators.yml": &_bintree_t{fabric8_online_team_colaborators_yml, map[string]*_bintree_t{
	}},
	"fabric8-online-team-openshift.yml": &_bintree_t{fabric8_online_team_openshift_yml, map[string]*_bintree_t{
	}},
	"fabric8-online-team-rolebindings.yml": &_bintree_t{fabric8_online_team_rolebindings_yml, map[string]*_bintree_t{
	}},
	"generate.go": &_bintree_t{generate_go, map[string]*_bintree_t{
	}},
}}
