Operation
The Go program in main.go is designed to analyze Terraform state files located in a specified directory. Here's a step-by-step breakdown of its operation:

Load Environment Variables: The program uses the godotenv package to load environment variables from a .env file. It specifically looks for the STATES_DIR variable, which should contain the path to the directory with the Terraform state files.

Walk Through Directory: The program uses filepath.Walk to traverse the directory specified by STATES_DIR. It processes each file in the directory, checking if the file has a .tfstate extension.

Read and Parse State Files: For each .tfstate file, the program reads the file's content and unmarshals the JSON data into a StateFile struct.

Organize State Files by Lineage: The state files are organized into a map (lineageMap) where the keys are the lineage identifiers, and the values are slices of StateFile structs.

Generate Report: The program creates a report file and writes information about the state files to it. This includes details such as the file name, serial number, lineage, timestamp, AWS caller identity, resource count, and changes in resources.

Configuration
The configuration for this program is provided through a .env file, which should contain the following environment variable:

STATES_DIR: The path to the directory containing the Terraform state files.
Output
The output of the program is a report file that summarizes the information about the state files. The report includes details such as the lineage, number of state files, timestamps, AWS caller identity, resource counts, and changes in resources.

Sample Report Output
Here is a sanitized example of what the report output might look like:

1 vulnerability
This sample report shows the directory being analyzed, the lineage of the state files, and detailed information about each state file, including timestamps, AWS caller identity, resource counts, and changes in resources.
