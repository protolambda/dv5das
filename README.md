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

## Revisiting the DAS network problem

Theory and crypto-libs are there, and the Merge is almost here (Catalyst, Rayonism, Amphora, Kintsugi, Kiln), now back to the networking problem.

The protocol has two independent identity layers, because of privacy and redundancy:
(everyone can setup their validators however, wherever and act however they like):
- consensus layer (validator entities signing stuff)
- network layer (nodes hosting validators)

To the consensus layer, DAS *only* needs to be accurate. The data that *is* available must not be ignored, or we get an unhealthy chain.

The network layer serves the above, requiring:
- Fast sample responses (keep accuracy for best case)
- Resilience (keep accuracy for worst case, e.g. DoS attacks and maintain randomness for security)
- Low resources (keep accuracy for lazy case, no excuses not to run it)

Also note that data-availability only requires data to be published to anyone unconditionally, 
not to persist it forever (incentivized honest nodes can keep storing it, as long as they can keep getting it).

Some ideas to create these properties:
- Reduce the number of hops for fast samples queries, and saving resources
- No small meshes as requester, to preserve sample randomness 
- Distribute the samples as much as possible, the uniform random sampling will load-balance the work
- Distributing helps offload privacy/DoS risk from data publishers
- Reduce the overhead per sample as much as possible, to save resources
- Rotate out data slow enough to serve queries, but fast enough to keep the relevant

Does this sound familiar? Discv5 solves a very similar problem!

What discv5 offers:
- The node tables can be large (16 x 16 = 256 records, plus replacements queue) and the network is not that much larger (rough estimate: 10k validating nodes, = approx 40 times the size).
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

Although the `TALKREQ`/`TALKRESP` are intended for initial handshakes / negotiation of extensions,
the communication in this protocol is infrequent and simple enough that `TALKREQ`/`TALKRESP` should work fine
(open for suggestions however).

The primary roles in the DHT are:
1. Seed the DHT with sample bundles as publisher
2. Fan-out the bundles as relayer
3. Query the DHT for samples as searcher
4. Respond to queries as host

And the samples all get hashed to an ID in the DHT: `H(fork_digest, randao_mix, data_id, sample_index) -> node_id`:
- `fork_digest` (`f`) - Bytes4, fork digest to separate networks
- `randao_mix` (`r`) - Bytes32, is randomness picked based on the sample time: this prevents mining of node identities close to a specific sample far in the future
- `data_id` (`x`) - Bytes48, is the KZG commitment of the data: samples are unique to the data they are part of
- `sample_index` (`i`) - uint64 (little endian), identifies the sample

The `fork_digest` and `randao_mix` can be determined based on time (slot `t`), and are not communicated or stored in the sample protocol.

Samples are stored in a key-value store.
Key: `t,x,i`  (`f` and `r` are determined based on slot `t`)
Value: sample data points and proof

#### Seeding

Generally we are not worried about the cost of the publisher as much:
 a block builder has more resources and has incentive to avoid orphaning their block because of missed/late publishing.

Before publishing the data, nodes should have the expected KZG commitment to all of it (proposer signed KZG commitment from builder on global topic).

The builder then generates the samples `0,...,N` and hashes them to their DHT location `H(f,r,x,0),...,H(f,r,x,N)`, and distributes them in bundles (see )

Each node that receives a sample can verify the sample proof against the global KZG commitment,
and limit spam by e.g. testing if the message is close enough to their identity.

Nodes that receive the samples can propagate them further to other nodes in the DHT radius of the sample: some redundancy is balances load and speeds up search.

Sybil attacks to capture the first distributed samples may be a problem, but a builder can score their peers, select long-lived peers, and distribute to extra nodes.

#### Querying

Each node (validating or not) can verify if data is available by making `k` random sample requests (`xs = [rand_range(0, N) for i in range(k)]`).

To request, the sample identity is hashed to a node identity `H(f,r,x,i)`. If the overlay has nearby nodes, then these are queried for the sample.
If not, then use regular discv5 search to find a node closer to `H(f,r,x,i)`.


### Peer score

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

### Sample Pruning

Samples are dropped after some locally configured expiry time.
Aggressive pruning may negatively affect the DAS peer score perceived by other nodes.

### Sample Gating

Samples pushed to the node by other peers are ignored, penalized, queued or accepted based on validation:
- Is the pushing peer banned? ignore.
- Do we not have the commitment? queue with timeout.
- Is the proof invalid? penalize.

Otherwise follow the sample distribution validation rules.

### Overlay

Since discv5 is also including non-ethereum nodes (IoT, alt networks, testnets, pretty much everything),
we need to filter down to just the nodes that participate: an overlay.

The discv5 overlay requires:
- No changes to discv5
- A `das` key-value pair in the node records. Used to filter between different DAS overlays. Absent for non-DAS nodes.
- Maintain more node records than a regular discv5 table
- Quick lookups of records close to any identity: DHT-search is the backup, not the default
- Ensure records are balanced and scored: an empty or poor-behaving sub-set in the DHT should be repaired with new records

#### Tree

Discv5 uses a XOR log distance: the more bits between two identities match, the closer they are together.

To find nodes for retrieval of samples, the overlay collection of records can be represented as binary tree:
- Each ID bit directs to the left or right sub-tree.
- Pair nodes can have empty left or right trees if there are no records
- Leaf nodes can be represented with extension bits (avoid many half-empty pair nodes)

Discv5 is extended, not modified: when a sub-tree is too small for reliable results, or when the nodes are misbehaving,
discv5 is used to find new nodes in the sub-tree that can serve samples.
This functions as a fallback during sampling, or as tree balancing during idle time.

#### Tree balancing

The scores of sub-trees add up:
```
Score(Pair(a, b)) = Score(a) + Score(b)
Score(Pair(nil, x)) = Score(Pair(x, nil)) = Score(x)
```

- Leaf nodes below a threshold score are pruned out.
- The weakest sub-tree is prioritized to grow with new nodes
- Nodes leak score over time when not accessed

### Sampling

#### Seeding samples

Samples fan-out to the DHT sub-trees with redundancy:
```
D = 8: fanout parameter, number of peers to distribute to (TODO: depends on network size)
P = 2: prefix bits per bundle step
```

After building a data-blob:
1. Extend to 2x size for error-recovery (only need 50% to recover)
2. Hash samples to sample identities
3. Bundle samples by ID prefix of `P` bits, e.g. `00`, `01`, `10`, `11`
4. Send bundles to `D` random nodes in corresponding subtrees (`TALKREQ`).
   Empty or single-sample bundles are not distributed further.

When receiving a bundle with prefix `XX` (a `TALKREQ`):
1. Validate the score of the sender is sufficient
2. If the bundle (or a superset of it) was received before, ignore it (execute no more steps) (remember `min` and `max` ID for bound checks)
   - TODO: if we want to track amount of repeated different seeders, we still need to validate the below conditions (extra cost)
3. Reject if the sample is too old
4. Reject if the bundle prefix does not match the local ID
5. Reject if the bundle contents do not all match the prefix
6. Verify each sample proof, reject if invalid
7. Split the bundle into smaller bundles with next `P` bits: `X00`, `X01`, `X10`, `X11`
8. Send smaller bundles to `D` random nodes in corresponding subtrees.
   Empty or single-sample bundles are not distributed further.
9. Increase score of bundle sender (TODO: )

#### Serving samples

On an incoming request for sample with ID inputs `t,x,i` (in a `TALKREQ`):
1. Validate and update rate-limit of the requesting peer, reject if exceeded
2. Reject if the requester has made the same request recently
3. Slightly decrease requester score if the sample is unknown, and do not respond
4. Increase the request count of the sample
5. Serve the request: send the sample (through a `TALKRESP`)

#### Retrieving samples

1. Find the closest record in the tree to the sample ID
2. Request the sample by sending the ID inputs `t,x,i` in a `TALKREQ`
3. On response:
   1. Verify proof of returned sample
   2. Increase score of node
4. On timeout:
   1. Decrease score of node
   2. Continue to step 2 with next closest node, give up after trying `R` closest nodes

#### Tree Repairing

During idle time, the tree is balanced through repairs:
1. select the weakest sub-tree for each depth
2. For each sub-tree, from deep to high up, run a DHT search for close nodes
3. Filter the retrieved node records for `das` support
4. Add nodes to the tree (do not overwrite existing leaf scores, do re-compute pair scores)



