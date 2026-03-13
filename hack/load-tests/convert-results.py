# collect all json files in given directory and convert them to a table line in markdown format
# Usage: python convert-results.py <directory>

import argparse
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
templates['logs-otel'] = (
    "\n"
    "| config | logs received | logs exported | logs queued | cpu | memory | no. restarts of gateway | no. restarts of generator "
    "|"
    "\n"
    "| --- | --- | --- | --- | --- | --- | --- "
    "|"
    "\n"
    "| single | {single[results][RECEIVED]} | {single[results][EXPORTED]} | {single[results][QUEUE]} | {single[results][CPU]} | {single[results][MEMORY]} | {single[results][RESTARTS_GATEWAY]} | {single[results][RESTARTS_GENERATOR]} "
    "|"
    "\n"
    "| batch | {batch[results][RECEIVED]} | {batch[results][EXPORTED]} | {batch[results][QUEUE]} | {batch[results][CPU]} | {batch[results][MEMORY]} | {batch[results][RESTARTS_GATEWAY]} | {batch[results][RESTARTS_GENERATOR]} "
    "|\n"
)
templates['logs-fluentbit'] = (
    "|        Version/Test "
    "|        Single Pipeline (ci-logs)        |                                          |                                 |                      |               "
    "|       Multi Pipeline (ci-logs-m)        |                                          |                                 |                      |               "
    "| Single Pipeline Backpressure (ci-logs-b) |                                          |                                 |                      |               "
    "| Multi Pipeline Backpressure (ci-logs-mb) |                                          |                                 |                      |               "
    "|\n"
    "|--------------------:"
    "|:---------------------------------------:|:----------------------------------------:|:-------------------------------:|:--------------------:|:-------------:"
    "|:---------------------------------------:|:----------------------------------------:|:-------------------------------:|:--------------------:|:-------------:"
    "|:----------------------------------------:|:----------------------------------------:|:-------------------------------:|:--------------------:|:-------------:"
    "|:----------------------------------------:|:----------------------------------------:|:-------------------------------:|:--------------------:|:-------------:"
    "|\n"
    "|                     "
    "| Input Bytes Processing Rate/sec (KByte) | Output Bytes Processing Rate/sec (KByte) | Input Log Records Processing Rate/sec | Output Log Records Processing Rate/sec | Filesystem Buffer Usage (KByte) | Pod Memory Usage(MB) | Pod CPU Usage "
    "| Input Bytes Processing Rate/sec (KByte) | Output Bytes Processing Rate/sec (KByte) | Input Log Records Processing Rate/sec | Output Log Records Processing Rate/sec | Filesystem Buffer Usage (KByte) | Pod Memory Usage(MB) | Pod CPU Usage "
    "| Input Bytes Processing Rate/sec (KByte)  | Output Bytes Processing Rate/sec (KByte) | Input Log Records Processing Rate/sec | Output Log Records Processing Rate/sec | Filesystem Buffer Usage (KByte) | Pod Memory Usage(MB) | Pod CPU Usage "
    "| Input Bytes Processing Rate/sec (KByte)  | Output Bytes Processing Rate/sec (KByte) | Input Log Records Processing Rate/sec | Output Log Records Processing Rate/sec | Filesystem Buffer Usage (KByte) | Pod Memory Usage(MB) | Pod CPU Usage "
    "|\n"
    "|                     "
    "| {single[results][RECEIVED]} | {single[results][EXPORTED]} | {single[results][RECEIVED_RECORDS]} | {single[results][EXPORTED_RECORDS]} | {single[results][QUEUE]} | {single[results][MEMORY]} | {single[results][CPU]} "
    "| {multi[results][RECEIVED]} | {multi[results][EXPORTED]} | {multi[results][RECEIVED_RECORDS]} | {multi[results][EXPORTED_RECORDS]} | {multi[results][QUEUE]} | {multi[results][MEMORY]} | {multi[results][CPU]} "
    "| {bp[results][RECEIVED]} | {bp[results][EXPORTED]} | {bp[results][RECEIVED_RECORDS]} | {bp[results][EXPORTED_RECORDS]} | {bp[results][QUEUE]} | {bp[results][MEMORY]} | {bp[results][CPU]} "
    "| {multi-bp[results][RECEIVED]} | {multi-bp[results][EXPORTED]} | {multi-bp[results][RECEIVED_RECORDS]} | {multi-bp[results][EXPORTED_RECORDS]} | {multi-bp[results][QUEUE]} | {multi-bp[results][MEMORY]} | {multi-bp[results][CPU]} "
    "|\n"
)
templates['traces'] = (
    "|       Version/Test "
    "| Single Pipeline (ci-traces) |                             |                     |                      |               "
    "| Multi Pipeline (ci-traces-m) |                             |                     |                      |              "
    "| Single Pipeline Backpressure (ci-traces-b) |                             |                     |                      |               "
    "| Multi Pipeline Backpressure (ci-traces-mb) |                             |                     |                      |               "
    "|\n"
    "|-------------------:"
    "|:---------------------------:|:---------------------------:|:-------------------:|:--------------------:|:-------------:"
    "|:----------------------------:|:---------------------------:|:-------------------:|:--------------------:|:-------------:"
    "|:------------------------------------------:|:---------------------------:|:-------------------:|:--------------------:|:-------------:"
    "|:------------------------------------------:|:---------------------------:|:-------------------:|:--------------------:|:-------------:"
    "|\n"
    "|                    "
    "| Receiver Accepted Spans/sec | Exporter Exported Spans/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage "
    "| Receiver Accepted Spans/sec  | Exporter Exported Spans/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage "
    "|        Receiver Accepted Spans/sec         | Exporter Exported Spans/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage "
    "|        Receiver Accepted Spans/sec         | Exporter Exported Spans/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage "
    "|\n"
    "|                    "
    "| {single[results][RECEIVED]} | {single[results][EXPORTED]} | {single[results][QUEUE]} | {single[results][MEMORY]} | {single[results][CPU]} "
    "| {multi[results][RECEIVED]} | {multi[results][EXPORTED]} | {multi[results][QUEUE]} | {multi[results][MEMORY]} | {multi[results][CPU]} "
    "| {bp[results][RECEIVED]} | {bp[results][EXPORTED]} | {bp[results][QUEUE]} | {bp[results][MEMORY]} | {bp[results][CPU]} "
    "| {multi-bp[results][RECEIVED]} | {multi-bp[results][EXPORTED]} | {multi-bp[results][QUEUE]} | {multi-bp[results][MEMORY]} | {multi-bp[results][CPU]} "
    "|\n"
)
templates['metrics'] = (
    "|       Version/Test "
    "| Single Pipeline (ci-metrics) |                              |                     |                      |               "
    "| Multi Pipeline (ci-metrics-m) |                              |                     |                      |               "
    "| Single Pipeline Backpressure (ci-metrics-b) |                              |                     |                      |               "
    "| Multi Pipeline Backpressure (ci-metrics-mb) |                              |                     |                      |               "
    "|\n"
    "|-------------------:"
    "|:----------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:"
    "|:-----------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:"
    "|:-------------------------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:"
    "|:-------------------------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:"
    "|\n"
    "|                    "
    "| Receiver Accepted Metric/sec | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage "
    "| Receiver Accepted Metric/sec  | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage "
    "|        Receiver Accepted Metric/sec         | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage "
    "|        Receiver Accepted Metric/sec         | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage "
    "|\n"
    "|                    "
    "| {single[results][RECEIVED]} | {single[results][EXPORTED]} | {single[results][QUEUE]} | {single[results][MEMORY]} | {single[results][CPU]} "
    "| {multi[results][RECEIVED]} | {multi[results][EXPORTED]} | {multi[results][QUEUE]} | {multi[results][MEMORY]} | {multi[results][CPU]} "
    "| {bp[results][RECEIVED]} | {bp[results][EXPORTED]} | {bp[results][QUEUE]} | {bp[results][MEMORY]} | {bp[results][CPU]} "
    "| {multi-bp[results][RECEIVED]} | {multi-bp[results][EXPORTED]} | {multi-bp[results][QUEUE]} | {multi-bp[results][MEMORY]} | {multi-bp[results][CPU]} "
    "|\n"
)
templates['self-monitor'] = (
    "| Version/Test "
    "| Default (ci-self-monitor) |                      |                        |                                  |                      |               "
    "|\n"
    "|-------------:"
    "|:-------------------------:|:--------------------:|:----------------------:|:--------------------------------:|:--------------------:|:-------------:"
    "|\n"
    "|              "
    "|    Scrape Samples/sec     | Total Series Created | WAL Storage Size/bytes | Head Chunk Storage Size in bytes | Pod Memory Usage(MB) | Pod CPU Usage "
    "|\n"
    "|              "
    "| {single[results][SCRAPESAMPLES]} | {single[results][SERIESCREATED]} | {single[results][WALSTORAGESIZE]} | {single[results][HEADSTORAGESIZE]} | {single[results][MEMORY]} | {single[results][CPU]} "
    "|\n"
)
templates['metricagent'] = (
    "|       Version/Test "
    "| Single Pipeline (ci-metric-ag) |                              |                     |                      |               "
    "| Single Pipeline Backpressure (ci-metric-ag-b) |                              |                     |                      |               "
    "|\n"
    "|-------------------:"
    "|:------------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:"
    "|:---------------------------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:"
    "|\n"
    "|                    "
    "|  Receiver Accepted Metric/sec  | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage "
    "|         Receiver Accepted Metric/sec          | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage "
    "|\n"
    "|                    "
    "| {single[results][RECEIVED]} | {single[results][EXPORTED]} | {single[results][QUEUE]} | {single[results][MEMORY]} | {single[results][CPU]} "
    "| {bp[results][RECEIVED]} | {bp[results][EXPORTED]} | {bp[results][QUEUE]} | {bp[results][MEMORY]} | {bp[results][CPU]} "
    "|\n"
)


# load all individual json files from the directories and combine them into a single dictionary
# the result looks like this:
# {
#   "metrics": { << test kind (metrics, selfmonitor, metricagent, etc.)
#     "single": {  << test type (single, multi, bp, multi-bp, etc.)
#       "test_name": "metrics",
#       "test_target": "metrics",
#       "max_pipeline": "false",
#       "nodes": [
#         "n1-standard-4",
#         "n1-standard-4"
#       ],
#       "backpressure_test": "false",
#       "results": {
#         "EXPORTED": "4477",
#         "RESTARTS_GATEWAY": "0",
#         "CPU": "1.5",
#         "RECEIVED": "4476",
#         "QUEUE": "0",
#         "TYPE": "metric",
#         "MEMORY": "247"
#       },
#       "test_duration": "1200",
#       "overlay": "",
#       "mode": "single"
#     }
#   }
# }
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
                    if data['overlay'] != "":
                        test_key.append(data['overlay'])
                    if len(test_key) == 0:
                        test_key.append('single')
                    new_data = defaultdict(str, data)
                    new_data['results'] = defaultdict(str, data['results'])
                    new_data['mode'] = '-'.join(test_key)
                    results[key]['-'.join(test_key)] = new_data
    return results

def print_results(results):
    # iterate over all test_targets
    for test_target, test_run in results.items():
        template = templates[test_target]
        try:
            # print the template with the data from the results
            print(template.format_map(results[test_target]))
        except KeyError as e:
            print("Template {} requires data for entry {}".format( test_target, e))



# main
if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-m','--markdown', action='store_true', help='Only output markdown table, skip debug prints')
    parser.add_argument('directories', nargs='+', help='Directories containing result files')

    args = parser.parse_args()
    results = load_results(args.directories)

    if not args.markdown:
        # debug print results
        print(json.dumps(results, indent=2))

    # print the results in markdown format
    print_results(results)
