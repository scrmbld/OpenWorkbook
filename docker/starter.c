// A setuid program for starting the code instance docker container
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <string.h>

int main(int argc, char **argv) {
	if (argc < 2) {
		fprintf(stderr, "Please pass the lua source directory as an argument\n");
		return 1;
	}

	char *sourceDir = malloc(1024*sizeof(char)); // 64 more just to be extra safe!

	if (argv[1][0] != '/') {
		sprintf(sourceDir, "%s/%s:/luasource", getenv("PWD"), argv[1]);
	}  else {
		sprintf(sourceDir, "%s:/luasource", argv[1]);
	}

	printf("%s\n", sourceDir);
	char *args[] = {"docker", "run", "-i", "-v", sourceDir, "runlua:latest", NULL};
	int code = execvp("/usr/bin/docker", args);

	free(sourceDir);
}
