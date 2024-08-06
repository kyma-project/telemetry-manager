# collect all json files in given directory and convert them to a table line in markdown format
# Usage: python convert-results.py <directory>

import json
import os
from collections import defaultdict

# the input json looks like this:
# {
#   "test_name": "No Name",
#   "test_target": "logs-otel",
#   "max_pipeline": "false",
#   "nodes": [
#     "c1.xlarge",
#     "c1.xlarge",
#     "c1.xlarge"
#   ],
#   "backpressure_test": "false",
#   "results": {
#     "EXPORTED": "7269",
#     "TYPE": "log",
#     "QUEUE": "null",
#     "CPU": "5.7",
#     "RECEIVED": "7258",
#     "MEMORY": "210",
#     "RESTARTS_GATEWAY": "0",
#     "RESTARTS_GENERATOR": "1"
#   },
#   "test_duration": "300"
# }


# templates for table line based on target_type
templates = {}
# templates['logs-otel'] = "|{bla} |"# {single[bla]} | {single[results][bla]} | {single[results][RECEIVED]} | {single[results][EXPORTED]} | {single[results][QUEUE]} | {single[results][CPU]} | {single[results][MEMORY]} | {single[results][RESTARTS_GATEWAY]} | {single[results][RESTARTS_GENERATOR]} |"
templates['logs-otel'] = "| {single[results][RECEIVED]} | {single[results][EXPORTED]} | {single[results][QUEUE]} | {single[results][CPU]} | {single[results][MEMORY]} | {single[results][RESTARTS_GATEWAY]} | {single[results][RESTARTS_GENERATOR]} |"
templates['logs-fluentbit'] = "| {single[results][RECEIVED]} | {single[results][EXPORTED]} | {single[results][QUEUE]} | {single[results][CPU]} | {single[results][MEMORY]} | {single[results][RESTARTS_GATEWAY]} | {single[results][RESTARTS_GENERATOR]} |"
templates['metrics'] = "| {single[results][RECEIVED]} | {single[results][EXPORTED]} | {single[results][QUEUE]} | {single[results][CPU]} | {single[results][MEMORY]} | {single[results][RESTARTS_GATEWAY]} | {single[results][RESTARTS_GENERATOR]} |"
templates['traces'] = "| {single[results][RECEIVED]} | {single[results][EXPORTED]} | {single[results][QUEUE]} | {single[results][CPU]} | {single[results][MEMORY]} | {single[results][RESTARTS_GATEWAY]} | {single[results][RESTARTS_GENERATOR]} |"
templates['selfmonitor'] = "| {single[results][RECEIVED]} | {single[results][EXPORTED]} | {single[results][QUEUE]} | {single[results][CPU]} | {single[results][MEMORY]} | {single[results][RESTARTS_GATEWAY]} | {single[results][RESTARTS_GENERATOR]} |"
templates['metricsagent'] = "| {single[results][RECEIVED]} | {single[results][EXPORTED]} | {single[results][QUEUE]} | {single[results][CPU]} | {single[results][MEMORY]} | {single[results][RESTARTS_GATEWAY]} | {single[results][RESTARTS_GENERATOR]} |"


def load_results(directories):
    results = defaultdict(dict)
    for directory in directories:
        for filename in os.listdir(directory):
            if filename.endswith(".json"):
                filenamePath = os.path.join(directory, filename)
                with open(filenamePath, mode='r') as f:
                    data = json.load(f)
                    # calculate a new key by combining test_target, max_pipeline and backpressure_test
                    key = data['test_target']
                    test_key = []
                    if data['max_pipeline'] == 'true':
                        test_key.append('multi')
                    if data['backpressure_test'] == 'true':
                        test_key.append('bp')
                    if len(test_key) == 0:
                        test_key.append('single')
                    new_data = defaultdict(str, data)
                    new_data['results'] = defaultdict(str, data['results'])
                    results[key]['-'.join(test_key)] = new_data
    return results

def print_results(results):
    # iterate over all test_targets
    for test_target, test_run in results.items():
        template = templates[test_target]
        print(template.format_map(results[test_target]))


# main
if __name__ == '__main__':
    import sys

    if len(sys.argv) < 2:
        print("Usage: python convert-results.py <directories>")
        sys.exit(1)

    # get all arguments
    directories = sys.argv[1:]
    results = load_results(directories)

    # debug print results
    print(json.dumps(results, indent=2))

    # print the results in markdown format
    print_results(results)
