import uuid

import pytest

from app.generator import CHANNELS, PAYMENT_METHODS, PRODUCTS, STATUSES, PurchaseGenerator

SKUS = {p["sku"] for p in PRODUCTS}


@pytest.fixture
def generator():
    return PurchaseGenerator(locale="pt_BR", seed=42)


def without_timestamp(event: dict) -> dict:
    return {k: v for k, v in event.items() if k != "event_timestamp"}


def test_purchase_has_expected_fields(generator):
    event = generator.purchase()

    assert set(event) == {
        "event_id", "event_type", "event_timestamp", "order_id", "status", "channel",
        "customer", "items", "payment", "currency", "subtotal", "discount", "shipping", "total",
    }
    assert set(event["customer"]) == {"customer_id", "name", "email", "city", "state"}
    assert event["event_type"] == "purchase"
    assert event["currency"] == "BRL"


def test_purchase_field_values_are_valid(generator):
    for event in generator.batch(200):
        assert uuid.UUID(event["event_id"]).version == 4
        assert event["order_id"].startswith("ORD-") and len(event["order_id"]) == 12
        assert event["customer"]["customer_id"].startswith("CUS-")
        assert "@" in event["customer"]["email"]
        assert event["status"] in STATUSES
        assert event["channel"] in CHANNELS
        assert event["payment"]["method"] in PAYMENT_METHODS
        assert 1 <= len(event["items"]) <= 4


def test_items_match_catalog(generator):
    prices = {p["sku"]: p["price"] for p in PRODUCTS}
    for event in generator.batch(200):
        for item in event["items"]:
            assert item["sku"] in SKUS
            assert item["unit_price"] == prices[item["sku"]]
            assert 1 <= item["quantity"] <= 5
            assert item["subtotal"] == round(item["unit_price"] * item["quantity"], 2)


def test_totals_are_consistent(generator):
    for event in generator.batch(200):
        assert event["subtotal"] == round(sum(i["subtotal"] for i in event["items"]), 2)
        assert 0 <= event["discount"] <= event["subtotal"] * 0.15 + 0.01
        assert event["total"] == round(event["subtotal"] - event["discount"] + event["shipping"], 2)


def test_free_shipping_above_threshold(generator):
    for event in generator.batch(200):
        if event["subtotal"] >= 199:
            assert event["shipping"] == 0.0
        else:
            assert 9.9 <= event["shipping"] <= 29.9


def test_installments_only_for_credit_card(generator):
    for event in generator.batch(200):
        payment = event["payment"]
        if payment["method"] == "credit_card":
            assert 1 <= payment["installments"] <= 12
        else:
            assert payment["installments"] == 1


def test_batch_returns_requested_count(generator):
    assert len(generator.batch(0)) == 0
    assert len(generator.batch(25)) == 25


def test_same_seed_generates_same_events():
    first = PurchaseGenerator(locale="pt_BR", seed=7).batch(10)
    second = PurchaseGenerator(locale="pt_BR", seed=7).batch(10)

    assert [without_timestamp(e) for e in first] == [without_timestamp(e) for e in second]


def test_different_seeds_generate_different_events():
    first = PurchaseGenerator(locale="pt_BR", seed=1).purchase()
    second = PurchaseGenerator(locale="pt_BR", seed=2).purchase()

    assert first["event_id"] != second["event_id"]


def test_event_ids_are_unique(generator):
    ids = [e["event_id"] for e in generator.batch(1000)]
    assert len(set(ids)) == len(ids)


def test_non_brazilian_locale_falls_back_to_state():
    event = PurchaseGenerator(locale="en_US", seed=1).purchase()
    assert event["customer"]["state"]
