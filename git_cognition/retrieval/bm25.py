from __future__ import annotations

from collections import Counter
from math import log
import re

TOKEN_RE = re.compile(r"[A-Za-z0-9_./-]+")


def tokenize(text: str) -> list[str]:
    return [match.group(0).lower() for match in TOKEN_RE.finditer(text or "")]


def rank_documents(query_text: str, documents: list[str]) -> list[tuple[int, float]]:
    if not documents:
        return []
    tokenized_docs = [tokenize(document) for document in documents]
    query_tokens = tokenize(query_text)
    if not query_tokens:
        return [(index, 0.0) for index in range(len(documents))]

    document_count = len(tokenized_docs)
    average_length = sum(len(doc) for doc in tokenized_docs) / max(document_count, 1)
    frequencies = [Counter(doc) for doc in tokenized_docs]
    document_frequencies: Counter[str] = Counter()
    for tokens in tokenized_docs:
        document_frequencies.update(set(tokens))

    k1 = 1.5
    b = 0.75
    rankings: list[tuple[int, float]] = []
    for index, doc_tokens in enumerate(tokenized_docs):
        score = 0.0
        doc_length = max(len(doc_tokens), 1)
        counts = frequencies[index]
        for token in query_tokens:
            freq = counts.get(token, 0)
            if not freq:
                continue
            df = document_frequencies.get(token, 0)
            idf = log((document_count - df + 0.5) / (df + 0.5) + 1.0)
            denom = freq + k1 * (1.0 - b + b * doc_length / max(average_length, 1.0))
            score += idf * (freq * (k1 + 1.0)) / denom
        rankings.append((index, score))
    rankings.sort(key=lambda item: item[1], reverse=True)
    return rankings
