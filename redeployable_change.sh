#!/bin/bash
#
# Check if there is a Change to something that is worth causing a tenant update
# 
# * Avoid redeploy when TEAM_VERSION change but templates don't (beyond versions)
#
EXIT_CODE=0
COMPS=(fabric8-online-team fabric8-online-jenkins fabric8-online-jenkins-quotas-oso fabric8-online-che fabric8-online-che-quotas-oso)
VERSION_CURR=`git show HEAD:TEAM_VERSION | tr -d "\n" | tr -d "\r"`
VERSION_PREV=`git show HEAD~1:TEAM_VERSION | tr -d "\n" | tr -d "\r"`

TMP=tmp

COMPARE_FILTER="grep -v \"version:\" | grep -v \"source-url\" | grep -v \"fabric8.io/git-branch\" | grep -v \"fabric8.io/metrics-path\" | grep -v \"fabric8.io/git-commit\""

function fetch {
	local url="http://central.maven.org/maven2/io/fabric8/online/packages/$1/$2/$1-$2-openshift.yml"
	curl -s --fail $url > $TMP/$1_$2.yml
	echo $TMP/$1_$2.yml
}

function compare {
	local f_a=`fetch $1 $2`
	local f_b=`fetch $1 $3`

	local result=`diff -B <(eval "cat $f_a | $COMPARE_FILTER") <(eval "cat $f_b | $COMPARE_FILTER")`
	echo $result
}

team_version_changed=`git diff HEAD~1 HEAD --name-only | grep TEAM_VERSION`

if [ ! -z "$team_version_changed" ]
then
	EXIT_CODE=1
	mkdir -p $TMP

	for comp in ${COMPS[@]}; do
		res=`compare $comp $VERSION_PREV $VERSION_CURR | tr -d "\n" | tr -d "\r"`
		if [ ! -z "$res" ]
		then
			EXIT_CODE=0
			echo "Changes $VERSION_PREV != $VERSION_CURR $comp"
			echo $res
			echo ""
		fi
	done
fi

exit $EXIT_CODE