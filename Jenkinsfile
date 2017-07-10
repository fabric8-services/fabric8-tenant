#!/usr/bin/groovy
@Library('github.com/fabric8io/fabric8-pipeline-library@master')
def utils = new io.fabric8.Utils()
def initServiceGitHash
def releaseVersion
goTemplate{
  dockerNode{
    ws {
      checkout scm

      if (utils.isCI()){
        goCI{
          githubOrganisation = 'fabric8-services'
          dockerOrganisation = 'fabric8'
          project = 'fabric8-tenant'
          dockerBuildOptions = '--file Dockerfile.deploy'
        }
      } else if (utils.isCD()){
        def v = goRelease{
          githubOrganisation = 'fabric8-services'
          dockerOrganisation = 'fabric8'
          project = 'fabric8-tenant'
          dockerBuildOptions = '--file Dockerfile.deploy'
        }

        releaseVersion = readFile('TEAM_VERSION').trim()
        initServiceGitHash = sh(script: 'git rev-parse HEAD', returnStdout: true).toString().trim()

        pushPomPropertyChangePR{
            propertyName = 'init-tenant.version'
            projects = [
                    'fabric8io/fabric8-platform'
            ]
            version = v
            containerName = 'go'
        }
      }
    }
    if (utils.isCD()){
      ws{
        container(name: 'go') {
          def flow = new io.fabric8.Fabric8Commands()
          sh 'chmod 600 /root/.ssh-git/ssh-key'
          sh 'chmod 600 /root/.ssh-git/ssh-key.pub'
          sh 'chmod 700 /root/.ssh-git'

          git 'git@github.com:openshiftio/saas.git'

          sh "git config user.email fabric8cd@gmail.com"
          sh "git config user.name fabric8-cd"

          def uid = UUID.randomUUID().toString()
          sh "git checkout -b versionUpdate${uid}"

          sh "sed -i -r 's/- hash: .*/- hash: ${initServiceGitHash}/g' dsaas-services/f8-tenant.yaml"

          def message = "Update tenants version to ${releaseVersion}"
          sh "git commit -a -m \"${message}\""
          sh "git push origin versionUpdate${uid}"

          def prId = flow.createPullRequest(message,'openshiftio/saas',"versionUpdate${uid}")
          flow.mergePR('openshiftio/saas',prId)
        }
      }
    }
  }
}
