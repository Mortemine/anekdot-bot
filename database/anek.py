from database import BASE
from sqlalchemy import ForeignKey
from sqlalchemy import Column, Integer, String


class Anek(BASE):
    __tablename__ = 'anekdots'
    anek_id = Column(Integer, primary_key=True, nullable=False, autoincrement=True)
    category = Column(Integer, ForeignKey("categories.cat_id"), nullable=False)
    text = Column(String, nullable=False)

    def __repr__(self) -> str:
        return f'Anek(id={self.anek_id!r}, category={self.category!r}, text={self.text!r})'


Anek.__table__.create(checkfirst=True, bind=BASE.metadata.bind)
