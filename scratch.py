sample_overhead = 70  # bytes   # proof, encoding, etc.
points_per_sample = 8
sample_content_size = 32*points_per_sample  # bytes       # data contents only

data_blob_size = 512 * 1024  # bytes
data_blob_interval = 12  # seconds
sample_requests_per_blob = 30   # number of samples a single node should do per blob    # TODO old num from V
node_count = 10_000
avg_peers_per_node = 2000

sample_size = sample_overhead + sample_content_size

data_avail_throughput = data_blob_size / data_blob_interval
sample_count_per_blob = data_blob_size / sample_content_size
sample_throughput = sample_count_per_blob / data_blob_interval   # samples / second (on chain, unextended)
outgoing_requests_rate = sample_requests_per_blob / data_blob_interval   # sample requests / second (single node)

outgoing_bandwidth = outgoing_requests_rate * sample_size   # incoming is expected to be the same (ignoring search overhead)

# data recovery by extending the data 2x for redundancy
extended_sample_throughput = sample_throughput * 2   # samples / second (on network, extended)

total_requests_rate = outgoing_requests_rate * node_count   # sample requests / second (all nodes combined)
avg_queries_per_sample = total_requests_rate / extended_sample_throughput  # sample requests / sample

# need to hide 50% for samples to not be recoverable
attack_hiding_factor = 0.50
# individual nodes may get adaptive answers on queries, we can only count on honest nodes.
# (we assume validators are evenly distributed between nodes here)
validating_nodes_ratio = 2.0 / 3.0

validating_nodes = node_count * validating_nodes_ratio

# TODO: implement monte carlo experiment to approximate sampling security (tricky by formula, because each node makes k distinct random requests, but nodes are otherwise randomly overlapping)
# Probability that a builder cannot answer to any 51% of the samples should be minimal. `k` can be adjusted, and the expected minimum honest node count requesting samples must be determined.

