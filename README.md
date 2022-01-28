# Implementing Data Availability Sampling (DAS)

There's a lot of history to unpack here. Vitalik posted about the "[Endgame](https://vitalik.ca/general/2021/12/06/endgame.html)":
where ethereum could be headed, assuming MEV is fundamental. Scaling data-availability is one of the fundamental endgame requirements.

DAS is not new though, there is a lot of background.

A brief summary:
- [KZG (Kate-Zaverucha-Goldberg)](https://www.iacr.org/archive/asiacrypt2010/6477178/6477178.pdf),  Aniket Kate, Gregory M. Zaverucha, Ian Goldberg, 2010
- [Reed-Solomon erasure code recovery in `n*log^2(n)` time with FFTs](https://ethresear.ch/t/reed-solomon-erasure-code-recovery-in-n-log-2-n-time-with-ffts/3039), Vitalik Buterin, Aug 2018
- [Fraud and Data Availability Proofs: Maximising Light Client Security and Scaling Blockchains with Dishonest Majorities](https://arxiv.org/abs/1809.09044), Mustafa Al-Bassam, Alberto Sonnino, Vitalik Buterin, Sep 2018
- [A note on data availability and erasure coding](https://github.com/ethereum/research/wiki/A-note-on-data-availability-and-erasure-coding), Vitalik Buterin, Sep 2018
- [Data availability checks](https://dankradfeist.de/ethereum/2019/12/20/data-availability-checks.html), Dankrad Feist, Dec 2019
- [With fraud-proof-free data availability proofs, we can have scalable data chains without committees](https://ethresear.ch/t/with-fraud-proof-free-data-availability-proofs-we-can-have-scalable-data-chains-without-committees/6725), Vitalik Buterin, Jan 2020
- [Fast amortized Kate proofs](https://github.com/khovratovich/Kate/blob/master/Kate_amortized.pdf), Dankrad Feist, Dmitry Khovratovich, Mar 2020
- [KZG polynomial commitments (explainer)](https://dankradfeist.de/ethereum/2020/06/16/kate-polynomial-commitments.html), Dankrad Feist, Jun 2020
- [2D data availability with Kate commitments](https://ethresear.ch/t/2d-data-availability-with-kate-commitments/8081), Vitalik Buterin, Oct 2020
- [Data availability sampling in practice](https://notes.ethereum.org/@vbuterin/r1v8VCULP), Vitalik Buterin, Oct 2020
- [Python code for the crypto part of sampling](https://github.com/ethereum/research/tree/master/kzg_data_availability), Dankrad Feist, Dec 2020

So **"wen data-availability sampling?"** Well, we need to solve the network layer still. This is a new proposal how it could work.

## Prototypes

The DAS-in-practice doc made a start, and a call with eth2 implementers was hosted in October 2020 to talk about implementation.
Then Dec 1 (2020) the beacon-chain launched, followed by holidays: no time from implementers for bleeding edge work end of 2020.

There were some educated guesses for an initial design direction though:
- Random sampling results in high subscription churn in GossipSub.
- Sampling needs to be very fast, DHTs are slow, right?
- GossipSub can handle attestation load, surely we can handle sampling if we avoid the churn?

And so the idea of a push-variant started: everyone rotates random subscriptions to sample-specific GossipSub topics slowly, to avoid churn.
Then seed the subnets via the shard topic (horizontal), and propagate samples to subscribers (vertical).

I was too optimistic, and started putting together a libp2p simulation of this GossipSub approach: [eth2-das](https://github.com/protolambda/eth2-das/).

The networking approach seemed ok-ish, but doubts started to arise around the altruism:
- How do we serve historical shard-data?
- How do we make sure sampling works past just the most recent things?
- If you propagate on 1000+ different topics, and the network is random and peer-fan is low, don't we eat a lot of undesired bandwidth?

With peer-scoring and banning, and frequent enough tiny tasks for the scores to make sense, you may have some chance it works.
Still, if the networking is too heavy, altruism with peer-scoring is not going to work. The prototype was then abandoned.

## KZG libs

Enough other problems to solve for implementers though, like using actual KZG proofs in prototypes.

So in January I ported over the python KZG10/FK20 code to optimized Go code: [Go-KZG](https://github.com/protolambda/go-kzg).
We formed a small working-group, and more ports, based on the Python and Go code, were created:
- [C-KZG](https://github.com/benjaminion/c-kzg), using [BLST](https://github.com/supranational/blst/) by Ben Edington (Teku team). 
  FK20, with optimized go code, then written in C, and hooked to the fasted BLS library. Amazing :clap:
- [JC-KZG](https://github.com/Nashatyrev/jc-kzg/), java bindings to C-KZG, developed by Anton Nashatyrev (Teku team).
- [Constantine (research code)](https://github.com/mratsim/constantine/tree/master/research/kzg_poly_commit), KZG FFTs and proofs ported to Nim by Mamy Ratsimbazafy (Nimbus team).
- [KZG in rust](https://github.com/sifraitech/kzg), Rust port by students of Saulius Grigaitis / SIFRAI-tech (Grandine team).

## Facing p2p network, again

Theory and crypto-libs are there, but now back to the networking problem.

Network challenges:
- No on-chain incentives in DAS. Unlike with attestations and proof of custody.
- Network and consensus validator identities are strictly separated because of privacy and redundancy:
  everyone can setup their validators however, wherever and act however they like.
- A huge amount of data to distribute, randomly access, and continuously update
- Randomness to maintain, vulnerable to DoS/bribes/etc. if sampling can be predicted and/or influenced.
- Organization to maintain, to distribute data more efficiently than literally propagating it to all participants

Challenging problem! But then hits the question: do you prioritize something that can have a similar impact, but is solvable faster?
Persisting means a back-and-forth between burn-out and bad ideas, work without a scope and impact is more often a distraction.

So then I focused on the Merge, the Rayonism merge devnets, sharding-spec updates (shard blobs, PBS with 64 shards, etc.), open rollup tech, the Amphora merge devnet, and then full-time Optimism.

You have to keep going, build other things when you can: ethereum is scaling because of pressures on usage, not just research.

## Revisiting Data-availability 

Optimism <3 data-availability: this is the most fundamental scaling problem rollups face. Time to revisit the problem!

The protocol has two independent identity layers:
- consensus layer (validator entities signing stuff)
- network layer (nodes hosting validators)

To the consensus layer, DAS *only* needs to be accurate. The data that *is* available must not be ignored, or we get an unhealthy chain.

The network layer serves the above, requiring:
- Fast sample responses (keep accuracy for best case)
- Resilience (keep accuracy for worst case, e.g. DoS attacks)
- Low resources (keep accuracy for lazy case, no excuses not to run it)

Also note that data-availability only requires data to be published to anyone unconditionally, 
not to persist it forever (incentivized honest nodes can keep storing it, as long as they can keep getting it).

Some ideas to create these properties:
- Reduce the number of hops for fast samples queries, and saving resources
- No small meshes, to preserve sample randomness 
- Distribute the samples as much as possible, the uniform random sampling will load-balance the work
- Distributing helps offload privacy/DoS risk from data publishers
- Reduce the overhead per sample as much as possible, to save resources
- Rotate out data slow enough to serve queries, but fast enough to keep the relevant

Does this sound familiar? Discv5 solves a very similar problem!

What discv5 offers:
- The node tables can be huge (up to 4096 records) and the network is not that much larger (rough estimate: 10k validating nodes).
  Number of hops will be low if some type of DHT search even works only a little bit.
- Only UDP communication: effective with resource to keep that many peers.
- Information distribution is similar: every record has a tiny signature to authenticate the data. Samples have a tiny proof to verify.
- Just like sending a record in a single packet, send a sample with a single packet.
- Discovery so far is used to retrieve records at random locations, and we do not need to advertise locations, they can be deterministic based on the sample hash
- Built to scale horizontally, and scale through participation

Caveat: discv5 is bigger than ethereum discovery, not all nodes may want to answer sample requests.
We'll need some type of overlay of ethereum-only discv5 nodes.


## Extending Discv5

But how does it fit discv5?

### Sampling

Discv5 offers `TALKREQ`/`TALKRESP` to extend the protocol with application-layer messaging.
Not all discv5 implementations support this yet, but it is part of the spec and we can start with it in Go.

There are two steps in sampling with a DHT:
1. Seed the DHT with samples as publisher
2. Query the DHT for samples as searcher

And a simple DHT hash function: `H(sample_id) -> node_id`. The `sample_id` may be a combination of index, block height,
and maybe beacon history or randomness (to avoid indefinite pre-mining data contents that skew all the samples to unfortunate DHT spots).

#### Seeding

Generally we are not worried about the cost of the publisher as much:
 a block builder has more resources and has incentive to avoid orphaning their block because of missed/late publishing.

Before publishing the data, nodes should have the expected KZG commitment to all of it (proposer signed KZG commitment from builder on global topic).

The builder then generates the samples `0,...,N`, hashes them to their DHT location `H(0),...,H(N)`, and sends them there (`N` UDP messages)

Each node that receives a sample can verify the sample proof against the global KZG commitment,
and limit spam by testing if the message is close enough to their identity.

Nodes that receive the samples can propagate them further to other nodes in the DHT radius of the sample: some redundancy is balances load and speeds up search.

Sybil attacks to capture the first distributed samples may be a problem, but a builder can score their peers, select long-lived peers, and distribute to extra nodes.

#### Querying

Each node (validating or not) can verify if data is available by making `k` random sample requests (`xs = [rand_range(0, N) for i in range(k)]`).

To request, the sample identity `x` is hashed to a node identity `H(x)`. If the overlay has nearby nodes, then these are queried for the sample.
If not, then use regular discv5 search to find a node closer to `H(x)`.


### Sample management

Although samples are tiny and distributed evenly, there is still a cost to holding them.
So we need to prune old samples (don't blow up memory), as well as gate new samples (guard against spam).

To prune and gate effectively, we need some context:
- peer score: do we trust the origin of this sample?
- saturation score: did many other peers see this sample?
- popularity score: did other peers request this sample often?
- responsibility score: how many times have other peers pushed this sample to us? (weight by peer score)
- sample time: at some point samples are too old
- randomness: attackers may try to beat the metrics, but it is much harder to beat occasional randomness

#### Peer score

Detected good/bad sample behaviors feed back to a scoring table of peers.

Think of:
- very bad: Stops serving a sample unexpectedly early
- very bad: Spams requests
- very bad: Seeds samples to us with bad proof
- bad: Requests when closer to sample than ourselves
- bad: Not behaving nice on discv5
- bad: Many identities behind same IP
- bad: Seeds outdated samples
- bad: Seeds samples that are too far away from local node ID
- bad: Duplicate sample query
- good: Serves sample requests
- good: Seeds samples to us (better if closer to our node ID, better if early)
- good: Stays consistently within sample rate limits
- very good: First to seed the sample to us

#### Saturation score

`saturation = seeders of sample / queries until sample found`

Samples with a higher saturation score can be preferred to drop, others will still be serving the sample.

#### Popularity score

`popularity = request count over rolling time window`

Some space should be allocated to retain the most popular samples: we may be the last serving them to the net.
If a popularity score is too high, the node can attempt to ralay the popular samples to others nearby,
to saturate the radius and distribute the response work.

Not all space is allocated, in case the popularity is faked (their peer scores should be negatively affected however).

#### Responsibility score

`responsibility = peer-score weighted avg. of push count of a sample`

If the other metrics check out, then allocate some space specifically for samples that other peers are giving us.

#### Pruning

Samples are stored in a priority-queue. Priorities are periodically updated, based on the scores and sample time.
The lowest priority items are first to be pruned.

#### Gating

Samples pushed to the node by other peers are ignored, penalized, queued or accepted based on validation:
- Is the pushing peer banned? ignore.
- Do we not have the commitment? queue.
- Is the proof invalid? penalize.

Otherwise accept, and update the metrics of both the pushing peer and the sample itself.

### Overlay

Since discv5 is also including non-ethereum nodes (IoT, alt networks, testnets, pretty much everything),
we need to filter down which nodes participate: an overlay.

The discv5 overlay requires:
- A `das` key-value pair in the node records. Used to filter between different DAS overlays. Absent for non-DAS nodes.
- Maintaining a nodes-table similar to the regular table, but filtered to only accept records with correct `das` entry
  - Possibly we can piggyback on the node revalidation of the regular table, to reduce cost of maintaining the overlay.



### Parameters scratchpad

Work in progress, figuring out the parameters of the network.

```python
sample_overhead = 70  # bytes   # proof, encoding, etc.
points_per_sample = 8
sample_content_size = 32*points_per_sample  # bytes       # data contents only

data_blob_size = 512 * 1024  # bytes
data_blob_interval = 12  # seconds
sample_requests_per_blob = 30   # number of samples a single node should do per blob    # TODO old num from V
node_count = 10_000
avg_peers_per_node = 2000

sample_size = sample_overhead + sample_content_size

data_avail_througput = data_blob_size / data_blob_interval
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


```


