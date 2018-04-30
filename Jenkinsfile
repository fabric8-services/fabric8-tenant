#!/usr/bin/groovy
@Library('github.com/fabric8io/fabric8-pipeline-library@master')
def utils = new io.fabric8.Utils()
def initServiceGitHash
def releaseVersion

// wait until the slower ci centos build has finished, then we know the image is in the devtools registry
node {
  checkout scm
  if (utils.isCD()){
    waitForCentosCIJob()
  }
}

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
          makeTarget = 'build test-unit-no-coverage-junit'
        }

        sh('mv /home/jenkins/go/src/github.com/fabric8-services/fabric8-tenant/tmp/junit.xml `pwd`')
        junit 'junit.xml'

      } else if (utils.isCD()){
        releaseVersion = goRelease{
          githubOrganisation = 'fabric8-services'
          dockerOrganisation = 'fabric8'
          project = 'fabric8-tenant'
          dockerBuildOptions = '--file Dockerfile.deploy'
        }

        initServiceGitHash = sh(script: 'git rev-parse HEAD', returnStdout: true).toString().trim()

        pushPomPropertyChangePR{
            propertyName = 'init-tenant.version'
            projects = [
                    'fabric8io/fabric8-platform'
            ]
            version = releaseVersion
            containerName = 'go'
        }
      }
    }
    if (true){
      ws{
        container(name: 'go') {
          def gitRepo = 'openshiftio/saas-openshiftio'
          def flow = new io.fabric8.Fabric8Commands()
          flow.setupGitSSH()

          def uid = UUID.randomUUID().toString()
          def branch = "versionUpdate${uid}"
          def message = "Update tenants version to ${releaseVersion}"

          sh """
             git clone git@github.com:${gitRepo} --depth=1
             cd  saas-openshiftio
             git checkout -b ${branch}
             sed -i -r 's/- hash: .*/- hash: ${initServiceGitHash}/g' dsaas-services/f8-tenant.yaml

             git commit -a -m "${message}"
             git push origin ${branch}
          """

          def prId = flow.createPullRequest(message, gitRepo, branch)
          flow.mergePR(gitRepo, prId)
        }
      }
    }
  }
}

def waitForCentosCIJob(){
  retry(5){
    // skip the merge commit and get the git sha for the actual change
    def commit = sh(script: "git --no-pager log -1 --no-merges  --pretty=format:\"%H\"", returnStdout: true).toString().trim()
    echo "Check CI centos has finished it's build for commit ${commit}"
    def jobListUrl = new URL("https://ci.centos.org/view/Devtools/job/devtools-fabric8-tenant-build-master/api/json?tree=builds[id,changeSet[items[commitId]]]")
    def rs = restGetURL{
      url = jobListUrl
    }
    def buildNumber
    for (build in rs.builds){
      if (build.changeSet){
        for (item in build.changeSet.items){
          if (commit == item.commitId){
            echo "matched commit ${commit}"
            buildNumber = build.id
            break
          }
        }
      }
      if (buildNumber){
        break
      }
    }
    if (!buildNumber){
        error "no CI centos build found yet for commit ${commit}"
        sleep 60
    }

    waitUntil {
      def buildUrl = new URL("https://ci.centos.org/view/Devtools/job/devtools-fabric8-tenant-build-master/${buildNumber}/api/json")
      def build = restGetURL{
        url = buildUrl
      }
      echo "CI centos build ${buildNumber} is ${build.result}"
      build.result == 'SUCCESS'
    }
  }
}
