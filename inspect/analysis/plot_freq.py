import sys
import matplotlib.pyplot as plt
from collections import Counter
import numpy as np
from load_protos import load_cooc, get_args


def plot_overlap_distribution(result):
    # flatten all overlap values into a single list
    overlaps = []
    for inner_dict in result.values():
        overlaps.extend(inner_dict.values())

    # count occurrences of each overlap value
    overlap_counts = Counter(overlaps)
    unique_overlaps, frequencies = zip(*sorted(overlap_counts.items()))

    # calculate the cumulative frequency and cumulative probability
    cumulative_frequencies = np.cumsum(frequencies)
    total = cumulative_frequencies[-1]  # total count of all overlaps
    cumulative_probabilities = (
        cumulative_frequencies / total
    )  # convert to probabilities

    # create subplots with shared x-axis
    fig, (ax1, ax2) = plt.subplots(
        2, 1, figsize=(10, 8), sharex=True, gridspec_kw={"height_ratios": [1, 1]}
    )

    # plot frequency distribution on the top plot
    ax1.plot(unique_overlaps, frequencies, marker=".", linestyle="-", color="b")
    ax1.set_ylabel("Frequency")
    ax1.set_xscale("log")  # log scale for x-axis
    ax1.grid(True, which="both", axis="y")
    ax1.set_title("Overlap (Cooccurrence) Values Distribution and CDF")

    # plot CDF on the bottom plot
    ax2.plot(
        unique_overlaps, cumulative_probabilities, marker=".", linestyle="-", color="r"
    )
    ax2.set_xlabel("Overlap Value in Nanoseconds (log scale)")
    ax2.set_xscale("log")  # log scale for x-axis
    ax2.set_ylabel("Cumulative Probability")
    ax2.grid(True, which="both", axis="y")

    plt.savefig("../images/freq.png", dpi=300)


if __name__ == "__main__":
    processed_args = get_args(sys.argv[1:])
    # filename=../data/ts1/ts1_cooccurrence.pb
    if filename := processed_args.get("filename"):
        loaded_cooc, max_overlap = load_cooc(filename)
        plot_overlap_distribution(loaded_cooc)
    else:
        sys.exit("argument 'filename' not provided (needed to load cooccurrence data)")
