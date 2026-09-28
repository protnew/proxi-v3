# P28/X5: dependency verification

Каркас verification-metadata.xml УДАЛЁН 2026-09-28: пустой components-block + verify-signatures=false
браковал первую же зависимость (AGP plugin marker POM) — Android-сборка была сломана с 22.09.

Правильный возврат (после решения X5 об артефакте):
  gradlew --write-verification-metadata sha256,pgp,md5 build
— сгенерирует заполненный metadata с реальными чексуммами/ключами, закоммитить результат.
