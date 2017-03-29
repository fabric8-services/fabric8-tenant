//xgo:generate sh -c "curl http://central.maven.org/maven2/io/fabric8/online/packages/fabric8-online-team/$TEAM_VERSION/fabric8-online-team-$TEAM_VERSION-openshift.yml | grep -v storage-class > fabric8-online-team-openshift.yml"
//xgo:generate sh -c "curl http://central.maven.org/maven2/io/fabric8/online/packages/fabric8-online-jenkins/$TEAM_VERSION/fabric8-online-jenkins-$TEAM_VERSION-openshift.yml | grep -v storage-class > fabric8-online-jenkins-openshift.yml"
//xgo:generate sh -c "curl http://central.maven.org/maven2/io/fabric8/online/packages/fabric8-online-che/$TEAM_VERSION/fabric8-online-che-$TEAM_VERSION-openshift.yml | grep -v storage-class > fabric8-online-che-openshift.yml"
package template
