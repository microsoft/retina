#!/bin/bash
if git log -1 --pretty=%G? | grep -q "G"; 
then
	echo "The latest commit is signed."
else
	echo "Error: The latest commit is not signed."
	exit 1;
fi 
