try{Database.query('SELECT CampaignId FROM MessagingSession LIMIT 1'); System.debug('CAP|success');}catch(Exception e){System.debug('CAP|error');}
