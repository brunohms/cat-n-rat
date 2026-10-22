import random
import uuid
from datetime import datetime, timezone

from faker import Faker

from .config import settings

PRODUCTS = [
    {"sku": "CAT-FOOD-01", "name": "Cat food 10kg", "category": "food", "price": 189.90},
    {"sku": "CAT-FOOD-02", "name": "Wet food pouches (box of 12)", "category": "food", "price": 42.50},
    {"sku": "CAT-TOY-01", "name": "Plush mouse", "category": "toys", "price": 14.90},
    {"sku": "CAT-TOY-02", "name": "Feather wand", "category": "toys", "price": 24.90},
    {"sku": "CAT-TOY-03", "name": "Laser pointer", "category": "toys", "price": 29.90},
    {"sku": "CAT-ACC-01", "name": "Scratching tower", "category": "accessories", "price": 249.00},
    {"sku": "CAT-ACC-02", "name": "Covered litter box", "category": "accessories", "price": 159.90},
    {"sku": "CAT-ACC-03", "name": "Cat litter 4kg", "category": "hygiene", "price": 34.90},
    {"sku": "CAT-ACC-04", "name": "Collar with bell", "category": "accessories", "price": 19.90},
    {"sku": "CAT-HLT-01", "name": "Dewormer", "category": "health", "price": 54.00},
]

PAYMENT_METHODS = ["credit_card", "debit_card", "pix", "boleto"]
STATUSES = ["created", "paid", "cancelled", "refunded"]
STATUS_WEIGHTS = [0.15, 0.75, 0.07, 0.03]
CHANNELS = ["web", "mobile_app", "store"]


class PurchaseGenerator:
    def __init__(self, locale: str = settings.locale, seed: int | None = settings.seed):
        self.fake = Faker(locale)
        self.rng = random.Random(seed)
        if seed is not None:
            self.fake.seed_instance(seed)

    def _item(self) -> dict:
        product = self.rng.choice(PRODUCTS)
        quantity = self.rng.randint(1, 5)
        return {
            "sku": product["sku"],
            "name": product["name"],
            "category": product["category"],
            "unit_price": product["price"],
            "quantity": quantity,
            "subtotal": round(product["price"] * quantity, 2),
        }

    def purchase(self) -> dict:
        items = [self._item() for _ in range(self.rng.randint(1, 4))]
        subtotal = round(sum(i["subtotal"] for i in items), 2)
        discount = round(subtotal * self.rng.choice([0, 0, 0, 0.05, 0.1, 0.15]), 2)
        shipping = 0.0 if subtotal >= 199 else round(self.rng.uniform(9.9, 29.9), 2)
        payment_method = self.rng.choice(PAYMENT_METHODS)

        return {
            "event_id": str(uuid.UUID(int=self.rng.getrandbits(128), version=4)),
            "event_type": "purchase",
            "event_timestamp": datetime.now(timezone.utc).isoformat(),
            "order_id": f"ORD-{self.rng.randint(0, 99_999_999):08d}",
            "status": self.rng.choices(STATUSES, weights=STATUS_WEIGHTS)[0],
            "channel": self.rng.choice(CHANNELS),
            "customer": {
                "customer_id": f"CUS-{self.rng.randint(1, 50_000):06d}",
                "name": self.fake.name(),
                "email": self.fake.email(),
                "city": self.fake.city(),
                "state": self.fake.estado_sigla() if hasattr(self.fake, "estado_sigla") else self.fake.state(),
            },
            "items": items,
            "payment": {
                "method": payment_method,
                "installments": self.rng.randint(1, 12) if payment_method == "credit_card" else 1,
            },
            "currency": "BRL",
            "subtotal": subtotal,
            "discount": discount,
            "shipping": shipping,
            "total": round(subtotal - discount + shipping, 2),
        }

    def batch(self, count: int) -> list[dict]:
        return [self.purchase() for _ in range(count)]
