List<Decimal> values = new List<Decimal>{2}; Decimal observed = values[0]++; System.debug('OBS|' + JSON.serialize(new List<Decimal>{observed,values[0]}));
