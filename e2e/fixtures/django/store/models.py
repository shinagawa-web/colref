from django.db import models
from base.models import TrackedItem


class Order(TrackedItem):
    total = models.DecimalField(max_digits=10, decimal_places=2)
