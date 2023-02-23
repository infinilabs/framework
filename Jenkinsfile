pipeline {

    agent {label 'master'}

    environment {
        CI = 'true'
    }
    stages {

       stage('build') {

        parallel {

        stage('Build Linux Packages') {

            agent {
                label 'linux'
            }

            steps {
                catchError(buildResult: 'SUCCESS', stageResult: 'FAILURE'){
                    sh 'cd /home/jenkins/go/src/infini.sh/framework && git fetch && git reset --hard origin/feature/jenkins'
                    sh 'cd /home/jenkins/go/src/infini.sh/framework && make test'
                }
            }
         }
        }
      }
    }
}
